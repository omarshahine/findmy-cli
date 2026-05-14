package findmy

import "testing"

func TestLookupStringsNormalizesLocaleTags(t *testing.T) {
	tests := []struct {
		lang      string
		peopleTab string
	}{
		{"fr-FR", "Personnes"},
		{"pt-PT", "Pessoas"},
		{"es-419", "Personas"},
		{"zh-Hant-TW", "聯絡人"},
		{"zh-Hans-CN", "联系人"},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			got := lookupStrings(tt.lang)
			if got.PeopleTab != tt.peopleTab {
				t.Fatalf("PeopleTab = %q, want %q", got.PeopleTab, tt.peopleTab)
			}
		})
	}
}

func TestSkipWordsIncludesLocalizedAndEnglishTabs(t *testing.T) {
	s := lookupStrings("fr-FR")
	skip := s.SkipWords()

	for _, word := range []string{"Personnes", "Appareils", "Objets", "Rechercher", "People", "Devices", "Items", "Search"} {
		if !skip[word] {
			t.Fatalf("SkipWords missing %q", word)
		}
	}
}
