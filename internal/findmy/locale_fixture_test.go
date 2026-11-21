package findmy

import "testing"

// These fixtures contain application display names only. They do not launch an
// app, query Spotlight, enumerate windows, or read a signed-in user's data.
func TestParseDisplayNameAllLocaleFixtures(t *testing.T) {
	for locale, app := range localeTable {
		t.Run(locale, func(t *testing.T) {
			for _, in := range []string{app.WindowOwner, app.WindowOwner + ".app", " \t" + app.WindowOwner + ".app\r\n"} {
				if got := parseDisplayName(in); got != app.WindowOwner {
					t.Errorf("parseDisplayName(%q) = %q, want %q", in, got, app.WindowOwner)
				}
			}
		})
	}
}

func TestParseDisplayNameFallbackFixtures(t *testing.T) {
	for _, in := range []string{"", " \t\r\n", "(null)", " (null)\n", "FindMy", "FindMy\n", "FindMy.app", " FindMy.app\r\n"} {
		if got := parseDisplayName(in); got != "" {
			t.Errorf("parseDisplayName(%q) = %q, want locale fallback", in, got)
		}
	}
}

func TestParseDisplayNameOnlyRemovesFinalBundleSuffix(t *testing.T) {
	for in, want := range map[string]string{
		"Fixture.app.app":     "Fixture.app",
		"Fixture.app Extra":   "Fixture.app Extra",
		"Fixture.application": "Fixture.application",
		"Fixture.APP":         "Fixture.APP",
	} {
		if got := parseDisplayName(in); got != want {
			t.Errorf("parseDisplayName(%q) = %q, want %q", in, got, want)
		}
	}
}
