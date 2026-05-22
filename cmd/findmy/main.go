package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/oshahine/findmy-cli/internal/findmy"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "people":
		runPeople(os.Args[2:])
	case "person":
		runPerson(os.Args[2:])
	case "devices":
		runDevices(os.Args[2:])
	case "device":
		runDevice(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `findmy — query Find My via UI scraping

Usage:
  findmy people  [--json] [--keep]
  findmy person  <name> [--json] [--keep] [--zoom]
  findmy devices [--json] [--keep]
  findmy device  <name> [--json] [--keep] [--zoom]

Flags:
  --json   emit JSON instead of a human table
  --keep   leave debug screenshots in /tmp/findmy-cli/
  --zoom   click matched row and OCR the detail pane`)
	os.Exit(2)
}

type runOpts struct {
	json bool
	keep bool
	zoom bool
}

// parseOpts splits args into known flags and positional args. Go's flag
// package stops at the first non-flag, but `findmy person Omar Shahine
// --json` puts the flag after positional args, so we pre-extract flags
// from anywhere in the slice.
func parseOpts(args []string) (runOpts, []string) {
	var o runOpts
	var positional []string
	for _, a := range args {
		switch a {
		case "--json", "-json":
			o.json = true
		case "--keep", "-keep":
			o.keep = true
		case "--zoom", "-zoom":
			o.zoom = true
		default:
			positional = append(positional, a)
		}
	}
	_ = flag.CommandLine
	return o, positional
}

func tmpDir() string {
	d := "/tmp/findmy-cli"
	_ = os.MkdirAll(d, 0o755)
	return d
}

func runPeople(args []string) {
	opts, _ := parseOpts(args)

	w, err := findmy.PreparePeople()
	must(err)
	shot := filepath.Join(tmpDir(), "people.png")
	must(findmy.Capture(w, shot))
	defer cleanup(shot, opts.keep)

	lines, err := findmy.OCR(shot)
	must(err)

	sidebarRightPx, textColMinPx := pixelLayout(w, shot)
	must(findmy.RequireSidebarVisible(lines, sidebarRightPx, "People"))
	people := findmy.ParsePeople(lines, sidebarRightPx, textColMinPx)

	if opts.json {
		emitJSON(people)
		return
	}
	if len(people) == 0 {
		fmt.Println("(no people found)")
		return
	}
	sort.SliceStable(people, func(i, j int) bool { return people[i].Name < people[j].Name })
	for _, p := range people {
		fmt.Printf("%s\n  %s", p.Name, p.Location)
		if p.Staleness != "" {
			fmt.Printf("  (%s)", p.Staleness)
		}
		if p.Distance != "" {
			fmt.Printf("  [%s]", p.Distance)
		}
		fmt.Println()
	}
}

func runDevices(args []string) {
	opts, _ := parseOpts(args)

	w, err := findmy.PrepareDevices()
	must(err)
	shot := filepath.Join(tmpDir(), "devices.png")
	must(findmy.Capture(w, shot))
	defer cleanup(shot, opts.keep)

	lines, err := findmy.OCR(shot)
	must(err)

	sidebarRightPx, textColMinPx := pixelLayout(w, shot)
	must(findmy.RequireSidebarVisible(lines, sidebarRightPx, "Devices"))
	devices := findmy.ParseDevices(lines, sidebarRightPx, textColMinPx)

	if opts.json {
		emitJSON(devices)
		return
	}
	if len(devices) == 0 {
		fmt.Println("(no devices found)")
		return
	}
	sort.SliceStable(devices, func(i, j int) bool { return devices[i].Name < devices[j].Name })
	for _, d := range devices {
		fmt.Printf("%s\n  %s", d.Name, d.Location)
		if d.Staleness != "" {
			fmt.Printf("  (%s)", d.Staleness)
		}
		if d.Distance != "" {
			fmt.Printf("  [%s]", d.Distance)
		}
		if d.Battery != "" {
			fmt.Printf("  {%s}", d.Battery)
		}
		fmt.Println()
	}
}

func runDevice(args []string) {
	opts, rest := parseOpts(args)
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: findmy device <name> [--json] [--keep] [--zoom]")
		os.Exit(2)
	}
	target := strings.ToLower(strings.Join(rest, " "))

	w, err := findmy.PrepareDevices()
	must(err)
	shot := filepath.Join(tmpDir(), "devices.png")
	must(findmy.Capture(w, shot))
	defer cleanup(shot, opts.keep)

	lines, err := findmy.OCR(shot)
	must(err)

	sidebarRightPx, textColMinPx := pixelLayout(w, shot)
	must(findmy.RequireSidebarVisible(lines, sidebarRightPx, "Devices"))
	devices := findmy.ParseDevices(lines, sidebarRightPx, textColMinPx)

	var match *findmy.Device
	for i := range devices {
		if strings.EqualFold(strings.TrimSpace(devices[i].Name), target) {
			match = &devices[i]
			break
		}
	}
	if match == nil {
		for i := range devices {
			if strings.Contains(strings.ToLower(devices[i].Name), target) {
				match = &devices[i]
				break
			}
		}
	}
	if match == nil {
		fmt.Fprintf(os.Stderr, "no device matching %q in sidebar\n", target)
		os.Exit(1)
	}
	if opts.zoom {
		nameLine, ok := findSidebarNameLine(lines, sidebarRightPx, textColMinPx, match.Name)
		if !ok {
			fmt.Fprintf(os.Stderr, "could not locate sidebar row for %q\n", match.Name)
			os.Exit(1)
		}
		detailShot := filepath.Join(tmpDir(), "device-detail.png")
		must(enrichWithDetailPane(w, shot, detailShot, nameLine, sidebarRightPx, opts.keep, func(precise, city, region, postal string) {
			match.PreciseAddress = precise
			match.City = city
			match.Region = region
			match.PostalCode = postal
		}))
	}

	if opts.json {
		emitJSON(match)
		return
	}
	fmt.Printf("%s\n  %s", match.Name, match.Location)
	if match.Staleness != "" {
		fmt.Printf("  (%s)", match.Staleness)
	}
	if match.Distance != "" {
		fmt.Printf("  [%s]", match.Distance)
	}
	if match.Battery != "" {
		fmt.Printf("  {%s}", match.Battery)
	}
	if match.PreciseAddress != "" {
		fmt.Printf("\n  %s", match.PreciseAddress)
		if match.City != "" || match.Region != "" || match.PostalCode != "" {
			fmt.Printf("\n  %s", strings.TrimSpace(strings.Join([]string{match.City, match.Region, match.PostalCode}, " ")))
		}
	}
	fmt.Println()
}

func runPerson(args []string) {
	opts, rest := parseOpts(args)
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: findmy person <name> [--json] [--keep] [--zoom]")
		os.Exit(2)
	}
	target := strings.ToLower(strings.Join(rest, " "))

	w, err := findmy.PreparePeople()
	must(err)
	shot := filepath.Join(tmpDir(), "people.png")
	must(findmy.Capture(w, shot))
	defer cleanup(shot, opts.keep)

	lines, err := findmy.OCR(shot)
	must(err)

	sidebarRightPx, textColMinPx := pixelLayout(w, shot)
	must(findmy.RequireSidebarVisible(lines, sidebarRightPx, "People"))
	people := findmy.ParsePeople(lines, sidebarRightPx, textColMinPx)

	var match *findmy.Person
	for i := range people {
		if strings.EqualFold(strings.TrimSpace(people[i].Name), target) {
			match = &people[i]
			break
		}
	}
	if match == nil {
		for i := range people {
			if strings.Contains(strings.ToLower(people[i].Name), target) {
				match = &people[i]
				break
			}
		}
	}
	if match == nil {
		fmt.Fprintf(os.Stderr, "no person matching %q in sidebar\n", target)
		os.Exit(1)
	}
	if opts.zoom {
		nameLine, ok := findSidebarNameLine(lines, sidebarRightPx, textColMinPx, match.Name)
		if !ok {
			fmt.Fprintf(os.Stderr, "could not locate sidebar row for %q\n", match.Name)
			os.Exit(1)
		}
		detailShot := filepath.Join(tmpDir(), "person-detail.png")
		must(enrichWithDetailPane(w, shot, detailShot, nameLine, sidebarRightPx, opts.keep, func(precise, city, region, postal string) {
			match.PreciseAddress = precise
			match.City = city
			match.Region = region
			match.PostalCode = postal
		}))
	}

	if opts.json {
		emitJSON(match)
		return
	}
	fmt.Printf("%s\n  %s", match.Name, match.Location)
	if match.Staleness != "" {
		fmt.Printf("  (%s)", match.Staleness)
	}
	if match.Distance != "" {
		fmt.Printf("  [%s]", match.Distance)
	}
	if match.PreciseAddress != "" {
		fmt.Printf("\n  %s", match.PreciseAddress)
		if match.City != "" || match.Region != "" || match.PostalCode != "" {
			fmt.Printf("\n  %s", strings.TrimSpace(strings.Join([]string{match.City, match.Region, match.PostalCode}, " ")))
		}
	}
	fmt.Println()
}

func findSidebarNameLine(lines []findmy.TextLine, sidebarRightPx, textColMinPx int, name string) (findmy.TextLine, bool) {
	target := strings.ToLower(strings.TrimSpace(name))
	var contains *findmy.TextLine
	for i := range lines {
		l := lines[i]
		txt := strings.TrimSpace(l.Text)
		if txt == "" {
			continue
		}
		if l.X+l.Width/2 >= sidebarRightPx {
			continue
		}
		if l.X < textColMinPx {
			continue
		}
		candidate := strings.ToLower(txt)
		if candidate == target {
			return l, true
		}
		if contains == nil && strings.Contains(candidate, target) {
			contains = &l
		}
	}
	if contains != nil {
		return *contains, true
	}
	return findmy.TextLine{}, false
}

func enrichWithDetailPane(w *findmy.Window, sidebarShotPath, detailShotPath string, clickLine findmy.TextLine, sidebarRightPx int, keep bool, apply func(precise, city, region, postal string)) error {
	clickX := clickLine.X + clickLine.Width/2
	clickY := clickLine.Y + clickLine.Height/2
	screenX, screenY := windowPointFromImagePoint(w, sidebarShotPath, clickX, clickY)
	if err := findmy.Click(screenX, screenY); err != nil {
		return fmt.Errorf("click matched row: %w", err)
	}

	time.Sleep(zoomDelay())

	if err := findmy.Capture(w, detailShotPath); err != nil {
		return err
	}
	defer cleanup(detailShotPath, keep)

	lines, err := findmy.OCR(detailShotPath)
	if err != nil {
		return err
	}
	precise, city, region, postal := findmy.ExtractDetailPaneAddress(lines, sidebarRightPx)
	if precise != "" || city != "" || region != "" || postal != "" {
		apply(precise, city, region, postal)
	}
	return nil
}

func zoomDelay() time.Duration {
	const fallback = 600 * time.Millisecond
	if raw := os.Getenv("FINDMY_ZOOM_DELAY_MS"); raw != "" {
		ms, err := strconv.Atoi(raw)
		if err == nil && ms > 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return fallback
}

// pixelLayout returns the sidebar-right and name-column-left thresholds in
// image pixels. The FindMy sidebar is ~340pt wide; the avatar column is
// ~100pt with the avatar circle centered around 50pt, so an 80pt cutoff
// drops centered avatar OCR fragments while admitting real name/location
// text that begins around 90pt. We use a float scale because some displays
// (e.g. a 4K dummy plug) report non-integer pixel-per-point ratios.
func pixelLayout(w *findmy.Window, imagePath string) (sidebarRightPx, textColMinPx int) {
	scale := imageScale(w, imagePath)
	return int(340 * scale), int(80 * scale)
}

func windowPointFromImagePoint(w *findmy.Window, imagePath string, px, py int) (int, int) {
	scale := imageScale(w, imagePath)
	return w.X + int(float64(px)/scale), w.Y + int(float64(py)/scale)
}

func imageScale(w *findmy.Window, imagePath string) float64 {
	scale := 2.0
	if info, err := imageSize(imagePath); err == nil && w.Width > 0 {
		if s := float64(info.W) / float64(w.Width); s >= 1 {
			scale = s
		}
	}
	return scale
}

type imgInfo struct{ W, H int }

func imageSize(path string) (imgInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return imgInfo{}, err
	}
	defer f.Close()
	cfg, _, err := decodeConfig(f)
	if err != nil {
		return imgInfo{}, err
	}
	return imgInfo{W: cfg.Width, H: cfg.Height}, nil
}

func emitJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func cleanup(path string, keep bool) {
	if !keep {
		_ = os.Remove(path)
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
