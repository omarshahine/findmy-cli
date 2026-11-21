//go:build darwin

package findmy

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Compile the production Swift window matcher against fixture dictionaries.
// Replace its single WindowServer query before compilation: this test must
// never enumerate real windows or need Screen Recording/Accessibility grants.
func TestWindowOwnerSwiftFixtures(t *testing.T) {
	source, err := os.ReadFile("../../helpers/findmy-helper/main.swift")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "func die(")
	end := strings.Index(text, "struct TextLine:")
	if start < 0 || end <= start {
		t.Fatal("cannot locate production Swift window matcher")
	}
	matcher := text[start:end]
	const liveQuery = "CGWindowListCopyWindowInfo([.optionAll], kCGNullWindowID)"
	if strings.Count(matcher, liveQuery) != 1 {
		t.Fatal("expected exactly one WindowServer query to replace")
	}
	matcher = strings.Replace(matcher, liveQuery, "fixtureWindows", 1)
	if strings.Contains(matcher, "CGWindowListCopyWindowInfo(") {
		t.Fatal("live WindowServer query remains in fixture source")
	}
	const fixtures = `import Foundation
import CoreGraphics

let fixtureWindows: Any = [
    [kCGWindowOwnerName as String: "Find My", kCGWindowNumber as String: 101,
     kCGWindowLayer as String: 0, kCGWindowIsOnscreen as String: true,
     kCGWindowBounds as String: ["X": 10, "Y": 20, "Width": 800, "Height": 600]],
    [kCGWindowOwnerName as String: "Localiser", kCGWindowNumber as String: 102,
     kCGWindowBounds as String: ["X": 10, "Y": 20, "Width": 800, "Height": 600]],
    [kCGWindowOwnerName as String: "Wo ist?", kCGWindowNumber as String: 103,
     kCGWindowBounds as String: ["X": 10, "Y": 20, "Width": 800, "Height": 600]],
    [kCGWindowOwnerName as String: "Find My Fixture Extra", kCGWindowNumber as String: 104,
     kCGWindowBounds as String: ["X": 10, "Y": 20, "Width": 800, "Height": 600]],
    [kCGWindowOwnerName as String: "Find My", kCGWindowNumber as String: 105]
] as [[String: Any]]

`
	dir := t.TempDir()
	swiftPath := filepath.Join(dir, "window-fixture.swift")
	binary := filepath.Join(dir, "window-fixture")
	program := fixtures + matcher + "\ncmdWindow(Array(CommandLine.arguments.dropFirst()))\n"
	if err := os.WriteFile(swiftPath, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("swiftc", "-o", binary, swiftPath).CombinedOutput(); err != nil {
		t.Fatalf("compile synthetic Swift fixture: %v\n%s", err, out)
	}
	for _, test := range []struct {
		display string
		id      int
	}{
		{"Find My", 101}, {"Find My.app\n", 101},
		{"Localiser.app\n", 102}, {"Wo ist?.app\n", 103},
		{"Find My Fixture Extra", 104},
		{"find my.app", 0}, {"Find.app", 0}, {"Unrelated.app", 0},
	} {
		t.Run(test.display, func(t *testing.T) {
			out, err := exec.Command(binary, "--owner", parseDisplayName(test.display)).Output()
			if err != nil {
				t.Fatal(err)
			}
			var windows []Window
			if err := json.Unmarshal(out, &windows); err != nil {
				t.Fatal(err)
			}
			if test.id == 0 {
				if len(windows) != 0 {
					t.Fatalf("matched unrelated fixture windows: %+v", windows)
				}
				return
			}
			if len(windows) != 1 || windows[0].WindowID != test.id {
				t.Fatalf("got %+v, want only fixture window %d", windows, test.id)
			}
		})
	}
}
