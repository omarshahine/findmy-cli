package findmy

import "testing"

func TestExtractDetailPaneAddressSplitsUSAddress(t *testing.T) {
	lines := []TextLine{
		{Text: "People", X: 20, Y: 20, Width: 80, Height: 20},
		{Text: "Omar Shahine", X: 760, Y: 110, Width: 220, Height: 30},
		{Text: "10001 NE 8th St", X: 760, Y: 155, Width: 230, Height: 24},
		{Text: "Bellevue, WA 98004", X: 760, Y: 185, Width: 230, Height: 24},
		{Text: "Directions", X: 720, Y: 250, Width: 110, Height: 24},
		{Text: "Notifications", X: 720, Y: 360, Width: 150, Height: 24},
	}

	precise, city, region, postal := ExtractDetailPaneAddress(lines, 680)

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

	precise, city, region, postal := ExtractDetailPaneAddress(lines, 680)

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

	precise, city, region, postal := ExtractDetailPaneAddress(lines, 680)

	if precise != "1 Apple Park Way" {
		t.Fatalf("precise = %q, want %q", precise, "1 Apple Park Way")
	}
	if city != "Cupertino" || region != "CA" || postal != "" {
		t.Fatalf("split = (%q, %q, %q), want (Cupertino, CA, empty)", city, region, postal)
	}
}
