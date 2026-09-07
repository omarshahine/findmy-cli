package findmy

import (
	"fmt"
	"image"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Ringing a device means driving the FindMy UI: find the row in the sidebar,
// open its detail card on the map, and click "Play Sound". Every step is OCR
// against a screenshot, so the code is written to retry rather than to assume.

// Scroll posts a continuous trackpad-style scroll at a screen point. Catalyst
// apps ignore discrete scroll-wheel events, so the helper sends phased pixel
// events instead. Positive dy scrolls up, negative down.
func Scroll(x, y, dy int) error {
	_, err := runHelper("scroll", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y), fmt.Sprintf("%d", dy))
	return err
}

// DeviceHit is a device located by scrolling the sidebar: just the name and
// where its row sits in the captured image, which is all RingDevice needs.
type DeviceHit struct {
	Name  string
	NameX int
	NameY int
}

// previousApp remembers who was frontmost before we activated FindMy, so a
// ring can hand the display back instead of leaving FindMy in the user's face.
var previousApp string

// RememberFrontApp records the frontmost app so RestoreUserSpace can return to
// it. It must be called BEFORE anything activates Find My -- PrepareDevices
// already does, and reading it afterwards just records Find My itself, which
// makes the restore a silent no-op.
func RememberFrontApp() {
	out, err := exec.Command("osascript", "-e",
		`tell application "System Events" to get bundle identifier of first process whose frontmost is true`).Output()
	if err != nil {
		previousApp = ""
		return
	}
	previousApp = strings.TrimSpace(string(out))
}

// RestoreUserSpace reactivates whatever was frontmost before the ring, which
// also switches macOS back to that app's Space.
func RestoreUserSpace() {
	if previousApp == "" || previousApp == "com.apple.findmy" {
		return
	}
	_ = exec.Command("osascript", "-e",
		fmt.Sprintf(`tell application id %q to activate`, previousApp)).Run()
}

// imageScaleFor derives the backing scale from a captured image. Capture uses
// `-o`, so the bitmap is exactly the window size times the scale.
func imageScaleFor(w *Window, imagePath string) float64 {
	scale := 2.0
	f, err := os.Open(imagePath)
	if err != nil {
		return scale
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || w.Width <= 0 {
		return scale
	}
	if s := float64(cfg.Width) / float64(w.Width); s >= 1 {
		return s
	}
	return scale
}

func captureAndOCR(w *Window, tmpDir string) ([]TextLine, error) {
	shot := filepath.Join(tmpDir, "snap.png")
	if err := Capture(w, shot); err != nil {
		return nil, err
	}
	defer os.Remove(shot)
	return OCR(shot)
}

// pollOCR captures and OCRs until match returns true or timeout elapses. It
// exists so the caller can exit as soon as the button appears instead of
// sleeping for the worst case on every attempt.
func pollOCR(w *Window, tmpDir string, timeout, interval time.Duration, match func([]TextLine) bool) []TextLine {
	deadline := time.Now().Add(timeout)
	shot := filepath.Join(tmpDir, "poll.png")
	for {
		if err := Capture(w, shot); err != nil {
			return nil
		}
		lines, err := OCR(shot)
		_ = os.Remove(shot)
		if err != nil {
			return nil
		}
		if match(lines) {
			return lines
		}
		if time.Now().After(deadline) {
			return nil
		}
		time.Sleep(interval)
	}
}

// playSoundLabels is the "Play Sound" button text in the languages FindMy
// ships. The button carries no accessibility identifier we can read from a
// screenshot, so the label is the only handle we have.
var playSoundLabels = []string{
	"play sound", "émettre un son", "emettre un son", "ton abspielen",
	"sound abspielen", "reproducir sonido", "riproduci suono", "emitir som",
	"geluid afspelen", "spela upp ljud", "afspil lyd", "spill av lyd",
	"toista ääni", "odtwórz dźwięk", "přehrát zvuk", "hang lejátszása",
	"ses çal", "воспроизвести звук", "サウンドを再生", "사운드 재생",
	"播放声音", "播放聲音", "เล่นเสียง", "phát âm thanh",
}

func findPlaySoundButton(lines []TextLine) *TextLine {
	for i := range lines {
		lower := strings.ToLower(strings.TrimSpace(lines[i].Text))
		if lower == "" {
			continue
		}
		for _, label := range playSoundLabels {
			if strings.Contains(lower, label) {
				return &lines[i]
			}
		}
	}
	return nil
}

// matchSidebarDevice picks the row for target out of one OCR frame. An exact
// name wins over a substring anywhere in the frame: "Omar's iPhone" and
// "Omar's iPhone 15" both contain the former, and ringing whichever happens to
// sit higher in the sidebar is not a coin flip worth taking.
func matchSidebarDevice(lines []TextLine, target string, sidebarRightPx int) *DeviceHit {
	targetLower := strings.ToLower(strings.TrimSpace(target))
	if targetLower == "" {
		return nil
	}
	var partial *DeviceHit
	for _, l := range lines {
		if l.X+l.Width/2 >= sidebarRightPx {
			continue // detail pane, not the sidebar
		}
		txt := strings.TrimSpace(l.Text)
		if txt == "" {
			continue
		}
		lower := strings.ToLower(txt)
		hit := &DeviceHit{Name: txt, NameX: l.X, NameY: l.Y}
		if lower == targetLower {
			return hit
		}
		if partial == nil && strings.Contains(lower, targetLower) {
			partial = hit
		}
	}
	return partial
}

// FindDeviceByScroll scrolls the Devices sidebar from the top, OCRing each
// frame, until a row matches target. The list is virtualized, so a device
// below the fold is invisible to a single capture -- scrolling is the only
// way to reach it.
func FindDeviceByScroll(w *Window, target, tmpDir string) (*DeviceHit, error) {
	if strings.TrimSpace(target) == "" {
		return nil, fmt.Errorf("no device name given")
	}

	sidebarX := w.X + 170 // horizontal center of the sidebar, in points
	sidebarY := w.Y + w.Height/2

	for i := 0; i < 20; i++ {
		_ = Scroll(sidebarX, sidebarY, 10)
	}
	time.Sleep(500 * time.Millisecond)

	shot := filepath.Join(tmpDir, "scroll-find.png")
	scale := 0.0
	for pass := 0; pass < 20; pass++ {
		if err := Capture(w, shot); err != nil {
			return nil, fmt.Errorf("capture: %w", err)
		}
		// Read the scale off the file while it still exists; OCR and removal
		// both come after.
		if scale == 0 {
			scale = imageScaleFor(w, shot)
		}
		lines, err := OCR(shot)
		_ = os.Remove(shot)
		if err != nil {
			return nil, fmt.Errorf("ocr: %w", err)
		}

		if hit := matchSidebarDevice(lines, target, int(340*scale)); hit != nil {
			return hit, nil
		}

		_ = Scroll(sidebarX, sidebarY, -5)
		time.Sleep(400 * time.Millisecond)
	}

	return nil, fmt.Errorf("device %q not found after scrolling the whole list", target)
}

// RingDevice opens a device's detail card and clicks "Play Sound".
//
// The sequence is empirical, not documented: click the sidebar row, wait for
// the map to settle, then double-click the map center to raise the pin popup
// and its detail card. The double-click is retried with growing delays --
// FindMy drops the second click while the map is still animating, and the
// retry ladder is what took this from roughly half the attempts to nearly all
// of them. With dryRun the button is located and reported but never clicked.
func RingDevice(w *Window, device *DeviceHit, tmpDir string, dryRun bool) error {
	if err := requirePermissions(true); err != nil {
		return err
	}

	_ = Activate()
	// Catalyst only routes synthetic clicks to a frontmost process.
	_ = exec.Command("osascript", "-e",
		`tell application "System Events" to tell process "FindMy" to set frontmost to true`).Run()
	time.Sleep(400 * time.Millisecond)

	shot := filepath.Join(tmpDir, "ring-scale.png")
	scale := 2.0
	if err := Capture(w, shot); err == nil {
		scale = imageScaleFor(w, shot)
		_ = os.Remove(shot)
	}

	// Fast path: the card may already be open from a previous run -- but only
	// if it is THIS device's card. A card left open for another device shows
	// its own Play Sound button, and clicking that rings the wrong thing.
	if lines, err := captureAndOCR(w, tmpDir); err == nil {
		if btn := findPlaySoundButton(lines); btn != nil && cardShowsDevice(lines, device.Name, int(340*scale)) {
			return clickOrDryRun(w, btn, scale, dryRun)
		}
	}

	screenX := w.X + int(float64(device.NameX)/scale)
	screenY := w.Y + int(float64(device.NameY)/scale) + 8
	if err := Click(screenX, screenY); err != nil {
		return fmt.Errorf("click device row: %w", err)
	}

	// Fixed wait: the map zoom animation has no completion signal we can read.
	time.Sleep(3 * time.Second)

	const sidebarPts = 340
	mapCenterX := w.X + sidebarPts + (w.Width-sidebarPts)/2
	mapCenterY := w.Y + w.Height/2

	var button *TextLine
	for attempt := 0; attempt < 5; attempt++ {
		_ = Click(mapCenterX, mapCenterY)
		time.Sleep(time.Duration(1000+attempt*200) * time.Millisecond)
		_ = Click(mapCenterX, mapCenterY)

		lines := pollOCR(w, tmpDir,
			time.Duration(2000+attempt*500)*time.Millisecond,
			300*time.Millisecond,
			func(lines []TextLine) bool { return findPlaySoundButton(lines) != nil })
		if lines != nil {
			button = findPlaySoundButton(lines)
			break
		}
	}

	if button == nil {
		return fmt.Errorf("'Play Sound' button not found -- the device may be offline, or its map pin was not clickable")
	}
	return clickOrDryRun(w, button, scale, dryRun)
}

// cardShowsDevice reports whether the open detail card names this device. The
// card renders to the right of the sidebar, so sidebar rows -- which always
// include the requested name once we have scrolled to it -- must not count as
// proof that the card itself belongs to that device.
func cardShowsDevice(lines []TextLine, name string, sidebarRightPx int) bool {
	want := strings.ToLower(strings.TrimSpace(name))
	if want == "" {
		return false
	}
	for _, l := range lines {
		if l.X < sidebarRightPx {
			continue // sidebar, not the card
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(l.Text)), want) {
			return true
		}
	}
	return false
}

// clickOrDryRun clicks the button, or reports where it would have clicked.
// The tappable icon sits above its label, hence the upward offset.
func clickOrDryRun(w *Window, btn *TextLine, scale float64, dryRun bool) error {
	btnX := w.X + int(float64(btn.X+btn.Width/2)/scale)
	btnY := w.Y + int(float64(btn.Y)/scale) - int(20/scale)
	if dryRun {
		fmt.Fprintf(os.Stderr, "dry-run: would click %q at screen (%d, %d)\n",
			strings.TrimSpace(btn.Text), btnX, btnY)
		return nil
	}
	if err := Click(btnX, btnY); err != nil {
		return fmt.Errorf("click play sound: %w", err)
	}
	return nil
}
