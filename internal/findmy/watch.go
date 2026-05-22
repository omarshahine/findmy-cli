package findmy

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

type WatchKind string

const (
	WatchPeople  WatchKind = "people"
	WatchDevices WatchKind = "devices"
	WatchItems   WatchKind = "items"
)

type WatchOptions struct {
	Kind     WatchKind
	Interval time.Duration
	Diff     bool
	JSON     bool
	Once     bool
	Out      io.Writer
}

type WatchEvent struct {
	Ts      time.Time            `json:"ts"`
	Event   string               `json:"event"`
	Kind    WatchKind            `json:"kind"`
	Record  any                  `json:"record,omitempty"`
	Changes map[string][2]string `json:"changes,omitempty"`
	Count   int                  `json:"count,omitempty"`
}

func Watch(opts WatchOptions) error {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.Interval == 0 {
		opts.Interval = 5 * time.Minute
	}
	if opts.Interval < 0 {
		return fmt.Errorf("interval must be greater than zero")
	}
	if err := validateWatchKind(opts.Kind); err != nil {
		return err
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	state := newWatchState(opts.Kind)
	first := true
	poll := func() error {
		records, err := pollOnce(opts.Kind)
		if err != nil {
			return err
		}
		events := state.diff(records)
		if opts.JSON {
			enc := json.NewEncoder(opts.Out)
			if len(events) == 0 && !opts.Diff {
				return enc.Encode(unchangedEvent(opts.Kind, len(records)))
			}
			for _, evt := range events {
				if err := enc.Encode(evt); err != nil {
					return err
				}
			}
			return nil
		}
		if !opts.Diff {
			if !first {
				fmt.Fprintln(opts.Out)
			}
			first = false
			fmt.Fprint(opts.Out, formatSnapshot(opts.Kind, records, time.Now().UTC()))
			return nil
		}
		for _, evt := range events {
			fmt.Fprintln(opts.Out, evt.Human())
		}
		return nil
	}

	if opts.Once {
		return poll()
	}
	if err := poll(); err != nil {
		fmt.Fprintf(os.Stderr, "watch poll error: %v (continuing)\n", err)
	}

	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := poll(); err != nil {
				fmt.Fprintf(os.Stderr, "watch poll error: %v (continuing)\n", err)
			}
		case <-sigCh:
			return nil
		}
	}
}

func validateWatchKind(kind WatchKind) error {
	switch kind {
	case WatchPeople, WatchDevices, WatchItems:
		return nil
	default:
		return fmt.Errorf("unknown watch kind %q", kind)
	}
}

func (e WatchEvent) Human() string {
	switch e.Event {
	case "added":
		return fmt.Sprintf("added %s: %s", e.Kind, humanRecord(e.Record))
	case "removed":
		return fmt.Sprintf("removed %s: %s", e.Kind, humanRecord(e.Record))
	case "updated":
		return fmt.Sprintf("updated %s: %s (%s)", e.Kind, humanRecord(e.Record), humanChanges(e.Changes))
	case "unchanged":
		return fmt.Sprintf("unchanged %s: %d records", e.Kind, e.Count)
	default:
		return fmt.Sprintf("%s %s: %s", e.Event, e.Kind, humanRecord(e.Record))
	}
}

func pollOnce(kind WatchKind) ([]watchRecord, error) {
	switch kind {
	case WatchPeople:
		w, err := PreparePeople()
		if err != nil {
			return nil, err
		}
		shot, lines, sidebarRightPx, textColMinPx, err := captureOCR(w, "people")
		defer os.Remove(shot)
		if err != nil {
			return nil, err
		}
		if err := RequireSidebarVisible(lines, sidebarRightPx, "People"); err != nil {
			return nil, err
		}
		return recordsForPeople(ParsePeople(lines, sidebarRightPx, textColMinPx)), nil
	case WatchDevices:
		w, err := PrepareDevices()
		if err != nil {
			return nil, err
		}
		shot, lines, sidebarRightPx, textColMinPx, err := captureOCR(w, "devices")
		defer os.Remove(shot)
		if err != nil {
			return nil, err
		}
		if err := RequireSidebarVisible(lines, sidebarRightPx, "Devices"); err != nil {
			return nil, err
		}
		return recordsForDevices(ParseDevices(lines, sidebarRightPx, textColMinPx)), nil
	case WatchItems:
		w, err := PrepareItems()
		if err != nil {
			return nil, err
		}
		shot, lines, sidebarRightPx, textColMinPx, err := captureOCR(w, "items")
		defer os.Remove(shot)
		if err != nil {
			return nil, err
		}
		if err := RequireSidebarVisible(lines, sidebarRightPx, "Items"); err != nil {
			return nil, err
		}
		return recordsForItems(ParseItems(lines, sidebarRightPx, textColMinPx)), nil
	default:
		return nil, fmt.Errorf("unknown watch kind %q", kind)
	}
}

func captureOCR(w *Window, stem string) (string, []TextLine, int, int, error) {
	shot := filepath.Join(watchTmpDir(), fmt.Sprintf("watch-%s.png", stem))
	if err := Capture(w, shot); err != nil {
		return shot, nil, 0, 0, err
	}
	lines, err := OCR(shot)
	if err != nil {
		return shot, nil, 0, 0, err
	}
	sidebarRightPx, textColMinPx := watchPixelLayout(w, shot)
	return shot, lines, sidebarRightPx, textColMinPx, nil
}

func watchTmpDir() string {
	d := "/tmp/findmy-cli"
	_ = os.MkdirAll(d, 0o755)
	return d
}

func watchPixelLayout(w *Window, imagePath string) (sidebarRightPx, textColMinPx int) {
	scale := watchImageScale(w, imagePath)
	return int(340 * scale), int(80 * scale)
}

func watchImageScale(w *Window, imagePath string) float64 {
	scale := 2.0
	if info, err := watchImageSize(imagePath); err == nil && w.Width > 0 {
		if s := float64(info.W) / float64(w.Width); s >= 1 {
			scale = s
		}
	}
	return scale
}

type watchImageInfo struct{ W, H int }

func watchImageSize(path string) (watchImageInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return watchImageInfo{}, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return watchImageInfo{}, err
	}
	return watchImageInfo{W: cfg.Width, H: cfg.Height}, nil
}

type watchRecord struct {
	Name      string
	Location  string
	Staleness string
	Distance  string
	Battery   string
	Record    any
}

func recordsForPeople(people []Person) []watchRecord {
	records := make([]watchRecord, 0, len(people))
	for _, p := range people {
		records = append(records, watchRecord{
			Name:      strings.TrimSpace(p.Name),
			Location:  p.Location,
			Staleness: p.Staleness,
			Distance:  p.Distance,
			Record:    p,
		})
	}
	return records
}

func recordsForDevices(devices []Device) []watchRecord {
	records := make([]watchRecord, 0, len(devices))
	for _, d := range devices {
		records = append(records, watchRecord{
			Name:      strings.TrimSpace(d.Name),
			Location:  d.Location,
			Staleness: d.Staleness,
			Distance:  d.Distance,
			Battery:   d.Battery,
			Record:    d,
		})
	}
	return records
}

func recordsForItems(items []Item) []watchRecord {
	records := make([]watchRecord, 0, len(items))
	for _, item := range items {
		records = append(records, watchRecord{
			Name:      strings.TrimSpace(item.Name),
			Location:  item.Location,
			Staleness: item.Staleness,
			Distance:  item.Distance,
			Battery:   item.Battery,
			Record:    item,
		})
	}
	return records
}

type watchState struct {
	kind     WatchKind
	lastSeen map[string]watchRecord
}

func newWatchState(kind WatchKind) *watchState {
	return &watchState{kind: kind, lastSeen: map[string]watchRecord{}}
}

func (s *watchState) diff(incoming []watchRecord) []WatchEvent {
	now := time.Now().UTC()
	next := map[string]watchRecord{}
	for _, record := range incoming {
		if record.Name == "" {
			continue
		}
		next[record.Name] = record
	}

	var added, removed, updated []WatchEvent
	for name, record := range next {
		previous, ok := s.lastSeen[name]
		if !ok {
			added = append(added, WatchEvent{
				Ts:     now,
				Event:  "added",
				Kind:   s.kind,
				Record: record.Record,
			})
			continue
		}
		changes := s.changes(previous, record)
		if len(changes) > 0 {
			updated = append(updated, WatchEvent{
				Ts:      now,
				Event:   "updated",
				Kind:    s.kind,
				Record:  record.Record,
				Changes: changes,
			})
		}
	}
	for name, record := range s.lastSeen {
		if _, ok := next[name]; !ok {
			removed = append(removed, WatchEvent{
				Ts:     now,
				Event:  "removed",
				Kind:   s.kind,
				Record: record.Record,
			})
		}
	}

	sortEventsByRecordName(added)
	sortEventsByRecordName(removed)
	sortEventsByRecordName(updated)

	events := make([]WatchEvent, 0, len(added)+len(removed)+len(updated))
	events = append(events, added...)
	events = append(events, removed...)
	events = append(events, updated...)
	s.lastSeen = next
	return events
}

func (s *watchState) changes(previous, current watchRecord) map[string][2]string {
	fields := []struct {
		name string
		old  string
		new  string
	}{
		{name: "location", old: previous.Location, new: current.Location},
		{name: "staleness", old: previous.Staleness, new: current.Staleness},
		{name: "distance", old: previous.Distance, new: current.Distance},
	}
	if s.kind != WatchPeople {
		fields = append(fields, struct {
			name string
			old  string
			new  string
		}{name: "battery", old: previous.Battery, new: current.Battery})
	}

	changes := map[string][2]string{}
	for _, field := range fields {
		if field.old != field.new {
			changes[field.name] = [2]string{field.old, field.new}
		}
	}
	if len(changes) == 0 {
		return nil
	}
	return changes
}

func unchangedEvent(kind WatchKind, count int) WatchEvent {
	return WatchEvent{
		Ts:    time.Now().UTC(),
		Event: "unchanged",
		Kind:  kind,
		Count: count,
	}
}

func sortEventsByRecordName(events []WatchEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		return eventRecordName(events[i]) < eventRecordName(events[j])
	})
}

func eventRecordName(evt WatchEvent) string {
	switch record := evt.Record.(type) {
	case Person:
		return record.Name
	case Device:
		return record.Name
	case Item:
		return record.Name
	case watchRecord:
		return record.Name
	default:
		return ""
	}
}

func formatSnapshot(kind WatchKind, records []watchRecord, ts time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s (%d)\n", ts.Format(time.RFC3339), kind, len(records))
	if len(records) == 0 {
		fmt.Fprintf(&b, "(no %s found)\n", kind)
		return b.String()
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].Name < records[j].Name
	})
	for _, record := range records {
		fmt.Fprintf(&b, "%s\n  %s", record.Name, record.Location)
		if record.Staleness != "" {
			fmt.Fprintf(&b, "  (%s)", record.Staleness)
		}
		if record.Distance != "" {
			fmt.Fprintf(&b, "  [%s]", record.Distance)
		}
		if record.Battery != "" {
			fmt.Fprintf(&b, "  {%s}", record.Battery)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func humanRecord(record any) string {
	switch r := record.(type) {
	case Person:
		return humanFields(r.Name, r.Location, r.Staleness, r.Distance, "")
	case Device:
		return humanFields(r.Name, r.Location, r.Staleness, r.Distance, r.Battery)
	case Item:
		return humanFields(r.Name, r.Location, r.Staleness, r.Distance, r.Battery)
	case watchRecord:
		return humanFields(r.Name, r.Location, r.Staleness, r.Distance, r.Battery)
	default:
		return fmt.Sprint(record)
	}
}

func humanFields(name, location, staleness, distance, battery string) string {
	var parts []string
	if location != "" {
		parts = append(parts, location)
	}
	if staleness != "" {
		parts = append(parts, "("+staleness+")")
	}
	if distance != "" {
		parts = append(parts, "["+distance+"]")
	}
	if battery != "" {
		parts = append(parts, "{"+battery+"}")
	}
	if len(parts) == 0 {
		return name
	}
	return fmt.Sprintf("%s - %s", name, strings.Join(parts, " "))
}

func humanChanges(changes map[string][2]string) string {
	if len(changes) == 0 {
		return "no field changes"
	}
	keys := make([]string, 0, len(changes))
	for key := range changes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		pair := changes[key]
		parts = append(parts, fmt.Sprintf("%s: %q -> %q", key, pair[0], pair[1]))
	}
	return strings.Join(parts, ", ")
}
