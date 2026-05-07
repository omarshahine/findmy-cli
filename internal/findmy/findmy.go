package findmy

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Window struct {
	PID      int    `json:"pid"`
	WindowID int    `json:"windowID"`
	Layer    int    `json:"layer"`
	Title    string `json:"title"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	OnScreen bool   `json:"onScreen"`
}

type TextLine struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	X          int     `json:"x"`
	Y          int     `json:"y"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
}

type Person struct {
	Name      string `json:"name"`
	Location  string `json:"location,omitempty"`
	Staleness string `json:"staleness,omitempty"`
	Distance  string `json:"distance,omitempty"`
}

func helper() string {
	if env := os.Getenv("FINDMY_HELPER"); env != "" {
		return env
	}
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "findmy-helper")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if path, err := exec.LookPath("findmy-helper"); err == nil {
		return path
	}
	return "findmy-helper"
}

func runHelper(args ...string) ([]byte, error) {
	cmd := exec.Command(helper(), args...)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

func Activate() error {
	script := `tell application "FindMy" to activate`
	return exec.Command("osascript", "-e", script).Run()
}

func SwitchTab(name string) error {
	script := fmt.Sprintf(
		`tell application "System Events" to tell process "FindMy" to click menu item %q of menu "View" of menu bar 1`,
		name,
	)
	return exec.Command("osascript", "-e", script).Run()
}

func MainWindow() (*Window, error) {
	out, err := runHelper("window", "--owner", "Find My")
	if err != nil {
		return nil, fmt.Errorf("helper window: %w", err)
	}
	var wins []Window
	if err := json.Unmarshal(out, &wins); err != nil {
		return nil, fmt.Errorf("decode windows: %w", err)
	}
	for _, w := range wins {
		if w.Layer == 0 && w.OnScreen && w.Height > 100 {
			return &w, nil
		}
	}
	return nil, fmt.Errorf("no visible FindMy window (open the app first)")
}

// Capture writes the FindMy window's content to dest using `screencapture -l`,
// which targets the window by ID and captures actual content rather than the
// screen rect. Region capture (`-R x,y,w,h`) would grab whatever is topmost at
// those coordinates and pollute the OCR with terminal/desktop content when
// FindMy isn't strictly frontmost.
//
// Capture fails with a friendly error when the display is asleep or the
// window's backing store hasn't been populated yet (Catalyst quirk after
// rapid focus changes). Both produce "could not create image from window"
// or a tiny all-black PNG.
func Capture(w *Window, dest string) error {
	cmd := exec.Command("/usr/sbin/screencapture", "-x", "-l", fmt.Sprintf("%d", w.WindowID), "-t", "png", dest)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return diagnoseCaptureFailure(err)
	}
	if info, err := os.Stat(dest); err == nil && info.Size() < 5_000 {
		return fmt.Errorf("captured image suspiciously small (%d bytes); display may be asleep — wake it with the keyboard", info.Size())
	}
	return nil
}

func diagnoseCaptureFailure(err error) error {
	if isDisplayAsleep() {
		return fmt.Errorf("display is asleep; wake it with the keyboard and re-run (%w)", err)
	}
	return fmt.Errorf("screencapture: %w (FindMy may not be fully painted; try again or click into FindMy first)", err)
}

func isDisplayAsleep() bool {
	out, err := exec.Command("ioreg", "-c", "IODisplayWrangler").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), `"CurrentPowerState" = 1`) ||
		strings.Contains(string(out), `"CurrentPowerState" = 0`)
}

func OCR(image string) ([]TextLine, error) {
	out, err := runHelper("ocr", image)
	if err != nil {
		return nil, fmt.Errorf("helper ocr: %w", err)
	}
	var lines []TextLine
	if err := json.Unmarshal(out, &lines); err != nil {
		return nil, fmt.Errorf("decode ocr: %w", err)
	}
	return lines, nil
}

func Click(x, y int) error {
	_, err := runHelper("click", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y))
	return err
}

// PreparePeople activates FindMy, raises it strictly frontmost (so the
// People sidebar is fully painted into the bitmap captured by `screencapture
// -l`), and selects the People tab via the View menu. Returns the window's
// metadata for capture targeting.
func PreparePeople() (*Window, error) {
	if err := Activate(); err != nil {
		return nil, err
	}
	time.Sleep(900 * time.Millisecond)
	frontScript := `tell application "System Events" to tell process "FindMy" to set frontmost to true`
	_ = exec.Command("osascript", "-e", frontScript).Run()
	_ = SwitchTab("People")
	time.Sleep(1100 * time.Millisecond)
	return MainWindow()
}

// ParsePeople groups OCR lines from the People sidebar into Person records.
// The sidebar is the leftmost ~340pt strip; the tab strip ends around y=160 in
// 2x retina pixels, and each row card is ~210px tall in retina coordinates.
// We pair lines by y-band: a name line is followed by a location/staleness
// line within 60px below it, and a distance line shares the band to the right.
func ParsePeople(lines []TextLine, sidebarRightPx int) []Person {
	type entry struct {
		line  TextLine
		inSB  bool
		inTab bool
	}
	rows := make([]entry, 0, len(lines))
	for _, l := range lines {
		if strings.TrimSpace(l.Text) == "" {
			continue
		}
		inSB := l.X+l.Width/2 < sidebarRightPx
		inTab := l.Y < 200
		rows = append(rows, entry{l, inSB, inTab})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].line.Y < rows[j].line.Y })

	skip := map[string]bool{
		"People": true, "Devices": true, "Items": true,
		"FaceTime": true, "Search": true, "+": true, "3D": true, "N": true,
	}

	var people []Person
	var current *Person
	for _, e := range rows {
		if !e.inSB || e.inTab {
			continue
		}
		txt := strings.TrimSpace(e.line.Text)
		if skip[txt] {
			continue
		}
		if current == nil {
			people = append(people, Person{Name: txt})
			current = &people[len(people)-1]
			continue
		}
		// Distance like "1,727 mi" sits to the right of the row top.
		// Heuristic: contains " mi" or " km" or " ft" or starts with digit and short.
		if isDistance(txt) {
			current.Distance = txt
			continue
		}
		// Location/staleness goes underneath the name. If the name has just
		// been added, this is its detail line.
		if current.Location == "" {
			loc, stale := splitLocationStaleness(txt)
			current.Location = loc
			current.Staleness = stale
			continue
		}
		// Anything past the detail line is the next person's name.
		people = append(people, Person{Name: txt})
		current = &people[len(people)-1]
	}
	return people
}

func isDistance(s string) bool {
	s = strings.ToLower(s)
	for _, suffix := range []string{" mi", " km", " ft", " m", " yd"} {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}
	return false
}

func splitLocationStaleness(s string) (location, staleness string) {
	if idx := strings.Index(s, "•"); idx >= 0 {
		return strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx+len("•"):])
	}
	return s, ""
}
