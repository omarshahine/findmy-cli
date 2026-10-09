package findmy

import "testing"

// macOS 27 sidebar geometry from a real 2x capture of a 1024x768pt window:
// four tab pills (People / Devices / Items / Me), a large title and section
// header in the left gutter, row text starting at 115-122px (about 59pt, right
// on the old fixed 60pt cutoff), "Nearby" in the distance column, and a map
// label to the right of the sidebar.
func macOS27DevicesLines() []TextLine {
	l := func(y, x, w int, text string) TextLine {
		return TextLine{Text: text, Confidence: 1, X: x, Y: y, Width: w, Height: 24}
	}
	return []TextLine{
		l(127, 59, 89, "People"),
		l(127, 196, 104, "Devices"),
		l(127, 354, 74, "Items"),
		l(125, 514, 41, "Me"),
		l(205, 32, 166, "Devices"),
		l(208, 565, 35, "+"),
		l(304, 29, 149, "My Devices"),
		l(389, 122, 273, "Alex's MacBook"),
		l(392, 663, 193, "+ Green Lake Park"),
		l(425, 118, 194, "Home • 2 min. ago"),
		l(517, 122, 184, "Alex's iPhone"),
		l(523, 532, 77, "Nearby"),
		l(553, 119, 130, "Home • Now"),
		l(645, 119, 258, "Alex's Work iPhone"),
		l(651, 532, 77, "Nearby"),
		l(684, 119, 181, "Home • Now • CD"),
		l(773, 119, 258, "Alex's AirPods Max"),
		l(778, 529, 80, "Nearby"),
		l(809, 118, 176, "Home • 4 hr. ago"),
		l(1285, 119, 351, "Alex's AirPods Max - Work"),
		l(1291, 544, 65, "7.3 mi"),
		l(1324, 119, 270, "Redmond, WA • 3 wk. ago"),
	}
}

func TestParseDevicesMacOS27Sidebar(t *testing.T) {
	// pixelLayout's fixed values at 2x: sidebar right 680px, text column 120px.
	got := ParseDevices(macOS27DevicesLines(), 680, 120)
	want := []Device{
		{Name: "Alex's MacBook", Location: "Home", Staleness: "2 min. ago"},
		{Name: "Alex's iPhone", Location: "Home", Staleness: "Now", Distance: "Nearby"},
		{Name: "Alex's Work iPhone", Location: "Home", Staleness: "Now", Distance: "Nearby"},
		{Name: "Alex's AirPods Max", Location: "Home", Staleness: "4 hr. ago", Distance: "Nearby"},
		{Name: "Alex's AirPods Max - Work", Location: "Redmond, WA", Staleness: "3 wk. ago", Distance: "7.3 mi"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d devices, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("device %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestDetectTextColumnMin(t *testing.T) {
	lines := macOS27DevicesLines()
	if got := DetectTextColumnMin(lines, 680, 160, 120); got != 106 {
		t.Errorf("macOS 27 cutoff = %d, want 106 (12px left of the 118px column)", got)
	}
	// Older layouts with text well right of the fixed cutoff keep it.
	older := []TextLine{
		{Text: "Alex", X: 240, Y: 300, Width: 100},
		{Text: "Home • Now", X: 241, Y: 330, Width: 100},
	}
	if got := DetectTextColumnMin(older, 680, 160, 120); got != 120 {
		t.Errorf("older layout cutoff = %d, want the 120px fallback", got)
	}
	// A lone stray fragment is not a column.
	if got := DetectTextColumnMin([]TextLine{{Text: "rk", X: 100, Y: 300, Width: 10}}, 680, 160, 120); got != 120 {
		t.Errorf("single-line cutoff = %d, want the 120px fallback", got)
	}
}

func TestIsDistanceNearby(t *testing.T) {
	if !isDistance("Nearby") {
		t.Error(`isDistance("Nearby") = false`)
	}
}
