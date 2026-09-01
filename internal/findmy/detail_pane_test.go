package findmy

import (
	"errors"
	"testing"
)

func TestExtractDetailPaneAddressSplitsUSAddress(t *testing.T) {
	lines := []TextLine{
		{Text: "People", X: 20, Y: 20, Width: 80, Height: 20},
		{Text: "Omar Shahine", X: 760, Y: 110, Width: 220, Height: 30},
		{Text: "10001 NE 8th St", X: 760, Y: 155, Width: 230, Height: 24},
		{Text: "Bellevue, WA 98004", X: 760, Y: 185, Width: 230, Height: 24},
		{Text: "Directions", X: 720, Y: 250, Width: 110, Height: 24},
		{Text: "Notifications", X: 720, Y: 360, Width: 150, Height: 24},
	}

	precise, city, region, postal, err := ExtractDetailPaneAddress(lines, 680, "Omar Shahine")

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if precise != "10001 NE 8th St" {
		t.Fatalf("precise = %q, want %q", precise, "10001 NE 8th St")
	}
	if city != "Bellevue" || region != "WA" || postal != "98004" {
		t.Fatalf("split = (%q, %q, %q), want (Bellevue, WA, 98004)", city, region, postal)
	}
}

func TestExtractDetailPaneAddressFallsBackForUnsplitAddress(t *testing.T) {
	lines := []TextLine{
		{Text: "Sadie Van Horn", X: 730, Y: 100, Width: 210, Height: 30},
		{Text: "10 Downing Street", X: 730, Y: 145, Width: 220, Height: 24},
		{Text: "London SW1A 2AA", X: 730, Y: 175, Width: 220, Height: 24},
		{Text: "5 mi away • Updated 2 min ago", X: 730, Y: 220, Width: 300, Height: 24},
	}

	precise, city, region, postal, err := ExtractDetailPaneAddress(lines, 680, "Sadie Van Horn")

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if precise != "10 Downing Street, London SW1A 2AA" {
		t.Fatalf("precise = %q, want fallback joined address", precise)
	}
	if city != "" || region != "" || postal != "" {
		t.Fatalf("split = (%q, %q, %q), want empty split fields", city, region, postal)
	}
}

func TestExtractDetailPaneAddressIgnoresSidebarAndButtons(t *testing.T) {
	lines := []TextLine{
		{Text: "Sidebar Person", X: 260, Y: 100, Width: 200, Height: 24},
		{Text: "5 mi", X: 600, Y: 100, Width: 60, Height: 24},
		{Text: "MacBook Pro", X: 760, Y: 110, Width: 220, Height: 30},
		{Text: "Battery: 82%", X: 760, Y: 150, Width: 140, Height: 24},
		{Text: "Play Sound", X: 760, Y: 180, Width: 120, Height: 24},
		{Text: "1 Apple Park Way", X: 760, Y: 220, Width: 220, Height: 24},
		{Text: "Cupertino, CA", X: 760, Y: 250, Width: 180, Height: 24},
	}

	precise, city, region, postal, err := ExtractDetailPaneAddress(lines, 680, "MacBook Pro")

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if precise != "1 Apple Park Way" {
		t.Fatalf("precise = %q, want %q", precise, "1 Apple Park Way")
	}
	if city != "Cupertino" || region != "CA" || postal != "" {
		t.Fatalf("split = (%q, %q, %q), want (Cupertino, CA, empty)", city, region, postal)
	}
}

// TestExtractDetailPaneAddressRejectsMapCanvas pins the macOS 26+ redesign
// (issue #13). The floating sidebar sits over a full-window map, so there is
// no detail pane at all and everything right of the sidebar is map furniture.
// The OCR fixture is a real `findmy person --zoom` capture on macOS 26.6.2,
// which previously produced precise_address = "Champaign Point, 3D".
func TestExtractDetailPaneAddressRejectsMapCanvas(t *testing.T) {
	lines := []TextLine{
		{Text: "People", X: 80, Y: 124, Width: 90, Height: 24},
		{Text: "Lora Shahine", X: 133, Y: 324, Width: 169, Height: 27},
		{Text: "Winston-Salem, NC • 3 min. ago", X: 136, Y: 353, Width: 300, Height: 24},
		{Text: "3D", X: 1900, Y: 40, Width: 40, Height: 24},
		{Text: "Champaign Point", X: 748, Y: 345, Width: 200, Height: 24},
		{Text: "Kirkland", X: 1128, Y: 595, Width: 160, Height: 30},
		{Text: "NE 85TH ST", X: 1400, Y: 590, Width: 180, Height: 20},
		{Text: "Lake Washington", X: 700, Y: 1390, Width: 190, Height: 24},
	}

	precise, city, region, postal, err := ExtractDetailPaneAddress(lines, 680, "Lora Shahine")

	if !errors.Is(err, ErrNoDetailPane) {
		t.Fatalf("err = %v, want ErrNoDetailPane", err)
	}
	if precise != "" || city != "" || region != "" || postal != "" {
		t.Fatalf("got (%q, %q, %q, %q), want all empty — map labels are not an address", precise, city, region, postal)
	}
}

// TestExtractDetailPaneAddressRejectsMapCallout is the second half of issue
// #13. Once the click lands, the redesigned FindMy answers with a callout
// pinned to the map: the entity's name over the same coarse location and
// staleness the sidebar already showed, surrounded by street labels. The
// callout header matches the entity, so header matching alone is not enough —
// the bullet-joined location must be rejected as an address, and the street
// labels must be rejected for sitting outside the header's column.
//
// Fixture is a real `findmy person "Lora Shahine" --zoom` capture on macOS
// 26.6.2, which produced precise_address =
// "Winston-Salem, NC • Now, WAREHAM LN, CHANCELLORSVILLE DR, HAGEN LN".
func TestExtractDetailPaneAddressRejectsMapCallout(t *testing.T) {
	lines := []TextLine{
		{Text: "BETHABARA PARK BLVD", X: 950, Y: 379, Width: 260, Height: 20},
		{Text: "Salemtowne", X: 890, Y: 512, Width: 150, Height: 24},
		{Text: "Lora Shahine", X: 1324, Y: 738, Width: 170, Height: 27},
		{Text: "Winston-Salem, NC • Now", X: 1327, Y: 773, Width: 240, Height: 22},
		{Text: "WAREHAM LN", X: 1132, Y: 862, Width: 160, Height: 20},
		{Text: "CHANCELLORSVILLE DR", X: 918, Y: 1056, Width: 280, Height: 20},
		{Text: "HAGEN LN", X: 1904, Y: 1147, Width: 130, Height: 20},
		{Text: "BULL RUN RD", X: 907, Y: 1342, Width: 160, Height: 20},
	}

	precise, city, region, postal, err := ExtractDetailPaneAddress(lines, 680, "Lora Shahine")

	if err != nil {
		t.Fatalf("err = %v, want nil (the callout header did match)", err)
	}
	if precise != "" || city != "" || region != "" || postal != "" {
		t.Fatalf("got (%q, %q, %q, %q), want all empty — the callout carries no street address", precise, city, region, postal)
	}
}

func TestLooksLikeAddressLine(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"10001 NE 8th St", true},
		{"1 Apple Park Way", true},
		{"Cupertino, CA", true},
		{"Bellevue, WA 98004", true},
		{"WAREHAM LN", true},
		// Substring matching used to accept these: "Winston" contains "st",
		// "Redmond" contains "rd", "Kirkland" contains "ln".
		{"Winston-Salem, NC • Now", false},
		{"Kirkland", false},
		{"Champaign Point", false},
		{"Salemtowne", false},
	}
	for _, c := range cases {
		if got := looksLikeAddressLine(c.line); got != c.want {
			t.Errorf("looksLikeAddressLine(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

// A detail pane that is genuinely on screen but has no address (offline
// device) is a different outcome from having no pane at all: no error, and
// nothing to apply.
func TestExtractDetailPaneAddressPaneWithoutAddress(t *testing.T) {
	lines := []TextLine{
		{Text: "Bike Shed Keys", X: 760, Y: 110, Width: 220, Height: 30},
		{Text: "No location found", X: 760, Y: 155, Width: 230, Height: 24},
		{Text: "Play Sound", X: 760, Y: 200, Width: 120, Height: 24},
	}

	precise, city, region, postal, err := ExtractDetailPaneAddress(lines, 680, "Bike Shed Keys")

	if err != nil {
		t.Fatalf("err = %v, want nil (the pane is present, it just has no address)", err)
	}
	if precise != "" || city != "" || region != "" || postal != "" {
		t.Fatalf("got (%q, %q, %q, %q), want all empty", precise, city, region, postal)
	}
}

func TestMatchesEntityHeader(t *testing.T) {
	cases := []struct {
		line, name string
		want       bool
	}{
		{"Omar Shahine", "Omar Shahine", true},
		{"omar shahine", "Omar Shahine", true},
		{"Omar Shahine…", "Omar Shahine", true},
		{"Omar Sunshine", "Omar Shahine", true}, // one OCR-mangled word of two
		{"Champaign Point", "Lora Shahine", false},
		{"3D", "Lora Shahine", false},
		{"Kirkland", "Omar's iPhone", false},
		{"", "Omar Shahine", false},
		{"Omar Shahine", "", false},
	}
	for _, c := range cases {
		if got := matchesEntityHeader(c.line, c.name); got != c.want {
			t.Errorf("matchesEntityHeader(%q, %q) = %v, want %v", c.line, c.name, got, c.want)
		}
	}
}
