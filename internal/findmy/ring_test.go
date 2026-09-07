package findmy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFindPlaySoundButtonMatchesLocalizedLabels(t *testing.T) {
	for _, label := range []string{"Play Sound", "Émettre un son", "Ton abspielen", "サウンドを再生"} {
		lines := []TextLine{
			{Text: "Omar's iPhone"},
			{Text: label, X: 100, Y: 400, Width: 80},
		}
		got := findPlaySoundButton(lines)
		if got == nil {
			t.Fatalf("label %q not matched", label)
		}
		if got.Text != label {
			t.Fatalf("matched %q, want %q", got.Text, label)
		}
	}
}

func TestFindPlaySoundButtonIgnoresUnrelatedText(t *testing.T) {
	lines := []TextLine{{Text: "Directions"}, {Text: "Notify When Found"}, {Text: ""}}
	if got := findPlaySoundButton(lines); got != nil {
		t.Fatalf("matched %q, want no match", got.Text)
	}
}

// The button label is OCR'd, so it arrives with whatever case and padding the
// renderer produced.
func TestFindPlaySoundButtonToleratesCaseAndPadding(t *testing.T) {
	lines := []TextLine{{Text: "  PLAY SOUND  "}}
	if findPlaySoundButton(lines) == nil {
		t.Fatal("uppercase padded label not matched")
	}
}

func TestImageScaleForFallsBackWhenImageMissing(t *testing.T) {
	w := &Window{Width: 1024}
	if got := imageScaleFor(w, filepath.Join(t.TempDir(), "nope.png")); got != 2.0 {
		t.Fatalf("got %v, want 2.0", got)
	}
}

func TestAliasesRoundTripAndResolveCaseInsensitively(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got := LoadAliases(); len(got) != 0 {
		t.Fatalf("expected no aliases in a fresh home, got %v", got)
	}
	if got := ResolveAlias("phone"); got != "phone" {
		t.Fatalf("unknown alias should pass through, got %q", got)
	}

	if err := SaveAliases(map[string]string{"Phone": "Omar's iPhone"}); err != nil {
		t.Fatal(err)
	}
	if got := ResolveAlias("PHONE"); got != "Omar's iPhone" {
		t.Fatalf("got %q, want the mapped device", got)
	}
	if got := ResolveAlias("  phone  "); got != "Omar's iPhone" {
		t.Fatalf("padded input got %q", got)
	}

	data, err := os.ReadFile(filepath.Join(home, ".config", "findmy-cli", "aliases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("aliases file is not valid JSON: %v", err)
	}
	if m["Phone"] != "Omar's iPhone" {
		t.Fatalf("on disk: %v", m)
	}
}

// A corrupt file must not take the CLI down; every command resolves aliases.
func TestLoadAliasesSurvivesCorruptFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "findmy-cli")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "aliases.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LoadAliases(); len(got) != 0 {
		t.Fatalf("got %v, want an empty map", got)
	}
}

func TestMatchSidebarDevicePrefersExactOverSubstring(t *testing.T) {
	// "Omar's iPhone 15" sits above the exact match; the exact one must win.
	lines := []TextLine{
		{Text: "Omar's iPhone 15", X: 60, Y: 100, Width: 200},
		{Text: "Omar's iPhone", X: 60, Y: 200, Width: 180},
	}
	got := matchSidebarDevice(lines, "Omar's iPhone", 680)
	if got == nil || got.Name != "Omar's iPhone" || got.NameY != 200 {
		t.Fatalf("got %+v, want the exact row at y=200", got)
	}
}

func TestMatchSidebarDeviceFallsBackToSubstring(t *testing.T) {
	lines := []TextLine{{Text: "Omar's iPhone 15", X: 60, Y: 100, Width: 200}}
	got := matchSidebarDevice(lines, "iphone", 680)
	if got == nil || got.Name != "Omar's iPhone 15" {
		t.Fatalf("got %+v, want the substring row", got)
	}
}

func TestMatchSidebarDeviceIgnoresDetailPane(t *testing.T) {
	// Same name, but rendered in the card to the right of the sidebar.
	lines := []TextLine{{Text: "Omar's iPhone", X: 900, Y: 100, Width: 180}}
	if got := matchSidebarDevice(lines, "Omar's iPhone", 680); got != nil {
		t.Fatalf("got %+v, want no sidebar match", got)
	}
}

func TestMatchSidebarDeviceEmptyTargets(t *testing.T) {
	lines := []TextLine{{Text: "Omar's iPhone", X: 60, Width: 180}}
	if got := matchSidebarDevice(lines, "   ", 680); got != nil {
		t.Fatalf("got %+v, want nil for a blank target", got)
	}
}

func TestCardShowsDeviceRequiresTheCardNotTheSidebar(t *testing.T) {
	sidebarOnly := []TextLine{{Text: "Omar's iPhone", X: 60, Width: 180}}
	if cardShowsDevice(sidebarOnly, "Omar's iPhone", 680) {
		t.Fatal("a sidebar row must not count as the card being open")
	}

	cardOpen := []TextLine{
		{Text: "Omar's iPhone", X: 60, Width: 180},
		{Text: "Omar's iPhone", X: 900, Width: 180},
	}
	if !cardShowsDevice(cardOpen, "Omar's iPhone", 680) {
		t.Fatal("card text to the right of the sidebar should count")
	}
}

// The fast path exists to skip re-navigation. It must not fire when the open
// card belongs to a different device, or a ring lands on the wrong one.
func TestCardShowsDeviceRejectsAnotherDevicesCard(t *testing.T) {
	lines := []TextLine{{Text: "Sarah's iPad", X: 900, Width: 160}, {Text: "Play Sound", X: 950}}
	if cardShowsDevice(lines, "Omar's iPhone", 680) {
		t.Fatal("another device's open card must not satisfy the fast path")
	}
}
