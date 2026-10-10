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

// watch scales the same constants as the one-shot commands; it once used an
// 80pt cutoff that dropped every macOS 27 row.
func TestParseDevicesMacOS27SidebarAtSharedLayout(t *testing.T) {
	got := ParseDevices(macOS27DevicesLines(), SidebarRightPt*2, TextColumnMinPt*2)
	if len(got) != 5 {
		t.Fatalf("got %d devices at the shared layout, want 5: %#v", len(got), got)
	}
}

func TestLocaleTableHasMacOS27Labels(t *testing.T) {
	for lang, s := range localeTable {
		if s.MeTab == "" || s.NearbyLabel == "" {
			t.Errorf("%s: MeTab=%q NearbyLabel=%q, want both set", lang, s.MeTab, s.NearbyLabel)
		}
	}
	if fr := lookupStrings("fr"); fr.MeTab != "Moi" || fr.NearbyLabel != "À proximité" {
		t.Errorf("fr labels = %q / %q", fr.MeTab, fr.NearbyLabel)
	}
}

// A scrolled macOS 27 Items sidebar: the top row is half under the header so
// only its status line shows, a section header sits in the left gutter, and
// shared items carry a third "Shared with" line. Before rows were grouped by
// vertical gap, each of these shifted every following row by one line.
func TestParseItemsMacOS27ScrolledWithSharedRows(t *testing.T) {
	l := func(y, x, w, h int, text string) TextLine {
		return TextLine{Text: text, Confidence: 1, X: x, Y: y, Width: w, Height: h}
	}
	lines := []TextLine{
		l(127, 59, 89, 28, "People"),
		l(127, 199, 98, 24, "Devices"),
		l(127, 357, 71, 21, "Items"),
		l(125, 514, 41, 26, "Me"),
		l(208, 565, 35, 32, "+"),
		l(211, 29, 119, 32, "Items"),
		l(284, 118, 209, 29, "Home • 4 min. ago •"),
		l(375, 118, 177, 36, "Blue Keys"),
		l(382, 517, 95, 24, "2,324 mi"),
		l(413, 118, 194, 24, "Home • 4 min. ago"),
		l(1018, 118, 194, 30, "Shed Keys"),
		l(1056, 119, 190, 21, "No location found"),
		l(1169, 26, 285, 27, "Items Shared With Me"),
		l(1253, 118, 215, 28, "Sam's Luggage"),
		l(1291, 119, 300, 24, "Washington, DC • 2 min. ago"),
		l(1324, 119, 193, 18, "Shared with Sam"),
		l(1413, 119, 220, 30, "Jo's Backpack"),
		l(1449, 118, 194, 24, "Home • 8 min. ago"),
		l(1481, 119, 187, 21, "Shared with Jo"),
	}
	got := ParseItems(lines, SidebarRightPt*2, TextColumnMinPt*2)
	want := []Item{
		{Name: "Blue Keys", Location: "Home", Staleness: "4 min. ago", Distance: "2,324 mi"},
		{Name: "Shed Keys", Location: "No location found"},
		{Name: "Sam's Luggage", Location: "Washington, DC", Staleness: "2 min. ago"},
		{Name: "Jo's Backpack", Location: "Home", Staleness: "8 min. ago"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}

// Vision sometimes reads "mi" as "ml"; the right-hand column is the distance
// whatever its text, so it must not become the location.
func TestParseDevicesDistanceByColumn(t *testing.T) {
	l := func(y, x, w int, text string) TextLine {
		return TextLine{Text: text, Confidence: 1, X: x, Y: y, Width: w, Height: 24}
	}
	lines := []TextLine{
		l(127, 59, 89, "People"), l(127, 196, 104, "Devices"), l(127, 354, 74, "Items"), l(125, 514, 41, "Me"),
		l(389, 119, 199, "Alex's AirPods"),
		l(395, 544, 68, "2,317 ml"),
		l(425, 119, 261, "Redmond, WA • 5 hr. ago"),
	}
	got := ParseDevices(lines, SidebarRightPt*2, TextColumnMinPt*2)
	want := Device{Name: "Alex's AirPods", Location: "Redmond, WA", Staleness: "5 hr. ago", Distance: "2,317 ml"}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %#v, want [%#v]", got, want)
	}
}

// Vision sometimes misses a row's name line; the orphaned status line must not
// become a row named "Washington, DC • 10 hr. ago".
func TestParseDevicesSkipsRowWithoutName(t *testing.T) {
	l := func(y, x, w int, text string) TextLine {
		return TextLine{Text: text, Confidence: 1, X: x, Y: y, Width: w, Height: 24}
	}
	lines := []TextLine{
		l(127, 59, 89, "People"), l(127, 196, 104, "Devices"), l(127, 354, 74, "Items"), l(125, 514, 41, "Me"),
		l(458, 119, 258, "Jo's MacBook Air"),
		l(464, 529, 80, "Nearby"),
		l(493, 118, 312, "Washington, DC • 41 min. ago"),
		l(591, 532, 77, "Nearby"),
		l(622, 119, 294, "Washington, DC • 10 hr. ago"),
		l(711, 115, 333, "Sam's AirPods"),
		l(749, 119, 187, "No location found"),
		// A second nameless row whose status has no "•".
		l(878, 119, 187, "No location found"),
	}
	got := ParseDevices(lines, SidebarRightPt*2, TextColumnMinPt*2)
	if len(got) != 2 || got[0].Name != "Jo's MacBook Air" || got[1].Name != "Sam's AirPods" {
		t.Fatalf("got %#v, want Jo's MacBook Air and Sam's AirPods only", got)
	}
}
