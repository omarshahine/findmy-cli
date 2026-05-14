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

type Permissions struct {
	ScreenRecording bool `json:"screenRecording"`
	Accessibility   bool `json:"accessibility"`
}

// CheckPermissions returns nil when both Screen Recording (for screencapture)
// and Accessibility / event-posting (for CGEvent clicks) are granted to the
// helper binary. It uses the helper's `permissions` subcommand, which probes
// via SCShareableContent rather than trusting CGPreflight*Access alone — TCC
// state is often stale for CLI binaries across rebuilds, and the preflight
// calls return false-negatives that would otherwise cause screencapture to
// hang or click to silently no-op.
func CheckPermissions() (Permissions, error) {
	out, err := runHelper("permissions")
	if err != nil {
		return Permissions{}, fmt.Errorf("helper permissions: %w", err)
	}
	var p Permissions
	if err := json.Unmarshal(out, &p); err != nil {
		return Permissions{}, fmt.Errorf("decode permissions: %w", err)
	}
	return p, nil
}

func requirePermissions(needClick bool) error {
	p, err := CheckPermissions()
	if err != nil {
		return err
	}
	var missing []string
	if !p.ScreenRecording {
		missing = append(missing, "Screen Recording")
	}
	if needClick && !p.Accessibility {
		missing = append(missing, "Accessibility")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"missing permission(s) for the host process: %s. Grant in System Settings → Privacy & Security → %s, then fully quit and relaunch this terminal (TCC is read once at process start).",
		strings.Join(missing, ", "), strings.Join(missing, " / "),
	)
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

// wakeDisplay nudges the display awake by holding a 3-second user-activity
// assertion. Needed for headless / closed-lid use with a dummy USB-C display:
// the dummy plug enables clamshell mode but macOS still idle-sleeps it, and
// WindowServer stops compositing when its only display is asleep — which
// makes `screencapture -l <windowID>` return "could not create image from
// window". We fire-and-forget; caffeinate self-terminates after 3s, which
// covers PreparePeople's ~2s of activate+sleep before the capture.
func wakeDisplay() {
	cmd := exec.Command("caffeinate", "-u", "-t", "3")
	if err := cmd.Start(); err == nil {
		go func() { _ = cmd.Wait() }()
	}
}

// PreparePeople activates FindMy, raises it strictly frontmost (so the
// People sidebar is fully painted into the bitmap captured by `screencapture
// -l`), and selects the People tab via the View menu. Returns the window's
// metadata for capture targeting. Fails fast if the host process is missing
// the Screen Recording grant, rather than letting screencapture hang.
func PreparePeople() (*Window, error) {
	if err := requirePermissions(false); err != nil {
		return nil, err
	}
	wakeDisplay()
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
// The sidebar layout (in image pixels) has three bands:
//
//	avatar:     x ≈   0–200   (round photo with initials — OCR noise lives here)
//	text:       x ≈ 240–550   (name and location/staleness)
//	distance:   x ≈ 580–700   ("1,971 mi" right-aligned to row top)
//
// We discard the avatar band entirely (it produces low-confidence fragments
// like "Is" or "rk" from initials and shadows that otherwise get misread as
// person names), then walk the remaining lines top-to-bottom.
func PeopleSidebarVisible(lines []TextLine, sidebarRightPx int) bool {
	seenPeople := false
	seenOtherTab := false
	for _, l := range lines {
		txt := strings.TrimSpace(l.Text)
		if txt != "People" && txt != "Devices" && txt != "Items" {
			continue
		}
		if l.Y > 220 {
			continue
		}
		if l.X+l.Width/2 >= sidebarRightPx {
			continue
		}
		if txt == "People" {
			seenPeople = true
		} else {
			seenOtherTab = true
		}
	}
	return seenPeople && seenOtherTab
}

func ParsePeople(lines []TextLine, sidebarRightPx, textColMinPx int) []Person {
	rows := make([]TextLine, 0, len(lines))
	effectiveSidebarRightPx := detectSidebarRight(lines, sidebarRightPx)
	rowStartY := detectPeopleRowStartY(lines, effectiveSidebarRightPx)
	for _, l := range lines {
		if strings.TrimSpace(l.Text) == "" {
			continue
		}
		if l.X+l.Width/2 >= effectiveSidebarRightPx {
			continue
		}
		if l.Y < rowStartY {
			continue
		}
		if l.X < textColMinPx {
			continue
		}
		rows = append(rows, l)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Y == rows[j].Y {
			return rows[i].X < rows[j].X
		}
		return rows[i].Y < rows[j].Y
	})
	rows = mergeWrappedContinuations(rows)

	skip := map[string]bool{
		"People": true, "Devices": true, "Items": true,
		"FaceTime": true, "Search": true, "+": true, "3D": true, "N": true,
	}

	people := make([]Person, 0)
	var current *Person
	for _, l := range rows {
		txt := strings.TrimSpace(l.Text)
		if skip[txt] {
			continue
		}
		if isDistance(txt) {
			if current != nil {
				current.Distance = txt
			}
			continue
		}
		if current == nil || current.Location != "" {
			people = append(people, Person{Name: txt})
			current = &people[len(people)-1]
			continue
		}
		loc, stale := splitLocationStaleness(txt)
		current.Location = loc
		current.Staleness = stale
	}
	return people
}

// mergeWrappedContinuations folds OCR lines that Vision split across two
// visual rows because of a long "City, ST • 2 min. ago" string. The
// telltale: the previous row contains the " • " separator and the next row
// is within ~35px below it and looks like a relative-time suffix.
func detectSidebarRight(lines []TextLine, fallbackRightPx int) int {
	maxTabRight := 0
	for _, l := range lines {
		txt := strings.TrimSpace(l.Text)
		if txt != "People" && txt != "Devices" && txt != "Items" {
			continue
		}
		if l.Y > 220 {
			continue
		}
		if r := l.X + l.Width; r > maxTabRight {
			maxTabRight = r
		}
	}
	if maxTabRight == 0 {
		return fallbackRightPx
	}
	// The segmented People/Devices/Items control sits inside the sidebar.
	// Its right edge is a better observed boundary than a fixed scaled point
	// width on compact Catalyst layouts where map labels start near x≈350px.
	observed := maxTabRight + 40
	if observed < fallbackRightPx {
		return observed
	}
	return fallbackRightPx
}

func detectPeopleRowStartY(lines []TextLine, sidebarRightPx int) int {
	const fallbackY = 120
	bottom := 0
	for _, l := range lines {
		txt := strings.TrimSpace(l.Text)
		if txt != "People" && txt != "Devices" && txt != "Items" {
			continue
		}
		if l.X+l.Width/2 >= sidebarRightPx {
			continue
		}
		if l.Y > 220 {
			continue
		}
		if b := l.Y + l.Height; b > bottom {
			bottom = b
		}
	}
	if bottom == 0 {
		return fallbackY
	}
	return bottom + 12
}

func mergeWrappedContinuations(rows []TextLine) []TextLine {
	out := make([]TextLine, 0, len(rows))
	for _, l := range rows {
		if n := len(out); n > 0 {
			prev := &out[n-1]
			gap := l.Y - (prev.Y + prev.Height)
			if gap < 12 && strings.Contains(prev.Text, "•") && looksLikeTimeSuffix(l.Text) {
				prev.Text = prev.Text + " " + strings.TrimSpace(l.Text)
				if l.Y+l.Height > prev.Y+prev.Height {
					prev.Height = (l.Y + l.Height) - prev.Y
				}
				continue
			}
		}
		out = append(out, l)
	}
	return out
}

func looksLikeTimeSuffix(s string) bool {
	t := strings.ToLower(strings.TrimSpace(s))
	for _, suffix := range []string{"min. ago", "min ago", "hr. ago", "hr ago", "sec. ago", "sec ago", "day ago", "days ago", "week ago", "weeks ago", "month ago", "months ago", "year ago", "years ago", "ago"} {
		if t == suffix || strings.HasSuffix(t, suffix) {
			return true
		}
	}
	return false
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
