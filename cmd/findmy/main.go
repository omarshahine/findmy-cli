package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/oshahine/findmy-cli/internal/findmy"
	"github.com/oshahine/findmy-cli/internal/findmy/ledger"
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
	case "log":
		runLog(os.Args[2:])
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
  findmy person  <name> [--json] [--keep]
  findmy devices [--json] [--keep]
  findmy device  <name> [--json] [--keep]
  findmy log     <name> [--kind=people|devices|items] [--since=DURATION] [--until=DURATION] [--limit=N] [--json]

Flags:
  --json    emit JSON instead of a human table
  --keep    leave debug screenshots in /tmp/findmy-cli/
  --no-log  skip writing people/devices observations to the history ledger`)
	os.Exit(2)
}

type runOpts struct {
	json  bool
	keep  bool
	noLog bool
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
		case "--no-log", "-no-log":
			o.noLog = true
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
	appendObservations("people", peopleObservations(people), opts.noLog)

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
	appendObservations("devices", deviceObservations(devices), opts.noLog)

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
		fmt.Fprintln(os.Stderr, "usage: findmy device <name> [--json] [--keep]")
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
	fmt.Println()
}

type logOpts struct {
	json  bool
	kind  string
	since time.Duration
	until time.Duration
	limit int
}

func runLog(args []string) {
	opts, rest, err := parseLogOpts(args)
	must(err)
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: findmy log <name> [--kind=people|devices|items] [--since=DURATION] [--until=DURATION] [--limit=N] [--json]")
		os.Exit(2)
	}

	now := time.Now()
	query := ledger.QueryOptions{
		Name:  strings.Join(rest, " "),
		Kind:  opts.kind,
		Limit: opts.limit,
	}
	if opts.since > 0 {
		query.Since = now.Add(-opts.since)
	}
	if opts.until > 0 {
		query.Until = now.Add(-opts.until)
	}

	l, err := ledger.Open("")
	must(err)
	defer l.Close()

	obs, err := l.Query(context.Background(), query)
	must(err)
	if opts.json {
		emitJSON(obs)
		return
	}
	if len(obs) == 0 {
		fmt.Println("(no observations found)")
		return
	}
	printLog(obs)
}

func parseLogOpts(args []string) (logOpts, []string, error) {
	opts := logOpts{limit: 100}
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--json" || a == "-json":
			opts.json = true
		case a == "--kind" || a == "-kind":
			i++
			if i >= len(args) {
				return opts, nil, fmt.Errorf("--kind requires a value")
			}
			opts.kind = args[i]
		case strings.HasPrefix(a, "--kind="):
			opts.kind = strings.TrimPrefix(a, "--kind=")
		case strings.HasPrefix(a, "-kind="):
			opts.kind = strings.TrimPrefix(a, "-kind=")
		case a == "--since" || a == "-since":
			i++
			if i >= len(args) {
				return opts, nil, fmt.Errorf("--since requires a duration")
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return opts, nil, fmt.Errorf("parse --since: %w", err)
			}
			opts.since = d
		case strings.HasPrefix(a, "--since="):
			d, err := time.ParseDuration(strings.TrimPrefix(a, "--since="))
			if err != nil {
				return opts, nil, fmt.Errorf("parse --since: %w", err)
			}
			opts.since = d
		case strings.HasPrefix(a, "-since="):
			d, err := time.ParseDuration(strings.TrimPrefix(a, "-since="))
			if err != nil {
				return opts, nil, fmt.Errorf("parse --since: %w", err)
			}
			opts.since = d
		case a == "--until" || a == "-until":
			i++
			if i >= len(args) {
				return opts, nil, fmt.Errorf("--until requires a duration")
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return opts, nil, fmt.Errorf("parse --until: %w", err)
			}
			opts.until = d
		case strings.HasPrefix(a, "--until="):
			d, err := time.ParseDuration(strings.TrimPrefix(a, "--until="))
			if err != nil {
				return opts, nil, fmt.Errorf("parse --until: %w", err)
			}
			opts.until = d
		case strings.HasPrefix(a, "-until="):
			d, err := time.ParseDuration(strings.TrimPrefix(a, "-until="))
			if err != nil {
				return opts, nil, fmt.Errorf("parse --until: %w", err)
			}
			opts.until = d
		case a == "--limit" || a == "-limit":
			i++
			if i >= len(args) {
				return opts, nil, fmt.Errorf("--limit requires a value")
			}
			limit, err := strconv.Atoi(args[i])
			if err != nil {
				return opts, nil, fmt.Errorf("parse --limit: %w", err)
			}
			opts.limit = limit
		case strings.HasPrefix(a, "--limit="):
			limit, err := strconv.Atoi(strings.TrimPrefix(a, "--limit="))
			if err != nil {
				return opts, nil, fmt.Errorf("parse --limit: %w", err)
			}
			opts.limit = limit
		case strings.HasPrefix(a, "-limit="):
			limit, err := strconv.Atoi(strings.TrimPrefix(a, "-limit="))
			if err != nil {
				return opts, nil, fmt.Errorf("parse --limit: %w", err)
			}
			opts.limit = limit
		default:
			positional = append(positional, a)
		}
	}
	if opts.kind != "" && opts.kind != "people" && opts.kind != "devices" && opts.kind != "items" {
		return opts, nil, fmt.Errorf("--kind must be people, devices, or items")
	}
	if opts.limit < 0 {
		return opts, nil, fmt.Errorf("--limit must be non-negative")
	}
	return opts, positional, nil
}

func appendObservations(kind string, obs []ledger.Observation, skip bool) {
	if skip || len(obs) == 0 {
		return
	}
	l, err := ledger.Open("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not open %s history ledger: %v\n", kind, err)
		return
	}
	defer l.Close()
	if err := l.Append(context.Background(), obs); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write %s history ledger: %v\n", kind, err)
	}
}

func peopleObservations(people []findmy.Person) []ledger.Observation {
	now := time.Now().UTC()
	obs := make([]ledger.Observation, 0, len(people))
	for _, p := range people {
		raw, _ := json.Marshal(p)
		obs = append(obs, ledger.Observation{
			Ts:        now,
			Kind:      "people",
			Name:      p.Name,
			Location:  p.Location,
			Staleness: p.Staleness,
			Distance:  p.Distance,
			Raw:       raw,
		})
	}
	return obs
}

func deviceObservations(devices []findmy.Device) []ledger.Observation {
	now := time.Now().UTC()
	obs := make([]ledger.Observation, 0, len(devices))
	for _, d := range devices {
		raw, _ := json.Marshal(d)
		obs = append(obs, ledger.Observation{
			Ts:        now,
			Kind:      "devices",
			Name:      d.Name,
			Location:  d.Location,
			Staleness: d.Staleness,
			Distance:  d.Distance,
			Battery:   d.Battery,
			Raw:       raw,
		})
	}
	return obs
}

func printLog(obs []ledger.Observation) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TS\tName\tLocation\tDistance\tBattery\tStaleness")
	for _, o := range obs {
		fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			o.Ts.Local().Format("2006-01-02 15:04:05"),
			o.Name,
			o.Location,
			o.Distance,
			o.Battery,
			o.Staleness,
		)
	}
	_ = w.Flush()
}

// Items logging should call appendObservations with kind "items" once the
// items command lands; this codebase currently only exposes people/devices.

func runPerson(args []string) {
	opts, rest := parseOpts(args)
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: findmy person <name> [--json] [--zoom]")
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
	fmt.Println()
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
