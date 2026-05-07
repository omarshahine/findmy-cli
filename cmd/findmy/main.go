package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
  findmy people [--json] [--keep]
  findmy person <name> [--json] [--keep]

Flags:
  --json   emit JSON instead of a human table
  --keep   leave debug screenshots in /tmp/findmy-cli/`)
	os.Exit(2)
}

type runOpts struct {
	json bool
	keep bool
}

func parseOpts(args []string) (runOpts, []string) {
	var o runOpts
	fs := flag.NewFlagSet("findmy", flag.ExitOnError)
	fs.BoolVar(&o.json, "json", false, "")
	fs.BoolVar(&o.keep, "keep", false, "")
	_ = fs.Parse(args)
	return o, fs.Args()
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

	sidebarRightPx := pixelSidebarBoundary(w)
	people := findmy.ParsePeople(lines, sidebarRightPx)

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

func runPerson(args []string) {
	opts, rest := parseOpts(args)
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: findmy person <name>")
		os.Exit(2)
	}
	target := strings.ToLower(strings.Join(rest, " "))

	w, err := findmy.PreparePeople()
	must(err)
	listShot := filepath.Join(tmpDir(), "people.png")
	must(findmy.Capture(w, listShot))
	defer cleanup(listShot, opts.keep)

	lines, err := findmy.OCR(listShot)
	must(err)

	var hit *findmy.TextLine
	for i := range lines {
		l := lines[i]
		if strings.ToLower(strings.TrimSpace(l.Text)) == target {
			hit = &lines[i]
			break
		}
	}
	if hit == nil {
		for i := range lines {
			l := lines[i]
			if strings.Contains(strings.ToLower(l.Text), target) {
				hit = &lines[i]
				break
			}
		}
	}
	if hit == nil {
		fmt.Fprintf(os.Stderr, "no person matching %q in sidebar\n", target)
		os.Exit(1)
	}

	clickX, clickY := windowPointFromImagePoint(w, listShot, hit.X+hit.Width/2, hit.Y+hit.Height/2)
	must(findmy.Click(clickX, clickY))
	time.Sleep(1500 * time.Millisecond)

	w2, err := findmy.MainWindow()
	must(err)
	detailShot := filepath.Join(tmpDir(), "person.png")
	must(findmy.Capture(w2, detailShot))
	defer cleanup(detailShot, opts.keep)

	detailLines, err := findmy.OCR(detailShot)
	must(err)

	detail := pickDetail(detailLines, w2)
	if opts.json {
		emitJSON(detail)
		return
	}
	fmt.Printf("%s\n", detail.Name)
	for _, l := range detail.Lines {
		fmt.Printf("  %s\n", l)
	}
}

type personDetail struct {
	Name  string   `json:"name"`
	Lines []string `json:"lines"`
}

func pickDetail(lines []findmy.TextLine, w *findmy.Window) personDetail {
	scaleX, _ := pixelScale(lines, w)
	sidebarRight := int(float64(340) * scaleX)
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].Y < lines[j].Y })
	var d personDetail
	for _, l := range lines {
		t := strings.TrimSpace(l.Text)
		if t == "" {
			continue
		}
		if l.X+l.Width/2 < sidebarRight {
			continue
		}
		if d.Name == "" {
			d.Name = t
			continue
		}
		d.Lines = append(d.Lines, t)
	}
	return d
}

func pixelScale(lines []findmy.TextLine, w *findmy.Window) (float64, float64) {
	maxX, maxY := 0, 0
	for _, l := range lines {
		if l.X+l.Width > maxX {
			maxX = l.X + l.Width
		}
		if l.Y+l.Height > maxY {
			maxY = l.Y + l.Height
		}
	}
	sx := 2.0
	sy := 2.0
	if w.Width > 0 && maxX > 0 {
		sx = float64(maxX) / float64(w.Width)
	}
	if w.Height > 0 && maxY > 0 {
		sy = float64(maxY) / float64(w.Height)
	}
	if sx < 0.5 || sx > 3.5 {
		sx = 2.0
	}
	if sy < 0.5 || sy > 3.5 {
		sy = 2.0
	}
	return sx, sy
}

func pixelSidebarBoundary(w *findmy.Window) int {
	const sidebarPt = 340
	return sidebarPt * 2
}

func windowPointFromImagePoint(w *findmy.Window, imagePath string, px, py int) (int, int) {
	scale := 2
	if info, err := imageSize(imagePath); err == nil && w.Width > 0 {
		scale = info.W / w.Width
		if scale < 1 {
			scale = 1
		}
	}
	return w.X + px/scale, w.Y + py/scale
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
