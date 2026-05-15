package findmy

import (
	"os"
	"os/exec"
	"strings"
	"sync"
)

// AppStrings holds the localized strings needed to drive FindMy.app via
// AppleScript menus and CGWindowList owner matching. The window owner name
// reported by CGWindowListCopyWindowInfo is the Finder display name, which
// macOS localizes per the system language (e.g. "Localiser" in French,
// "Wo ist?" in German). Menu item names follow the same localization.
//
// Override auto-detection with FINDMY_LANG=fr (or any key in localeTable).
type AppStrings struct {
	WindowOwner  string   // CGWindowList owner (e.g. "Find My", "Localiser")
	ViewMenu     string   // View menu bar item (e.g. "View", "Présentation")
	PeopleTab    string   // People tab in View menu
	DevicesTab   string   // Devices tab
	ItemsTab     string   // Items tab
	SearchLabel  string   // Search field label in sidebar
	TimeSuffixes []string // patterns for wrapped-line merging (see looksLikeTimeSuffix)
}

var (
	appStrings     *AppStrings
	appStringsOnce sync.Once
)

// GetAppStrings returns the localized strings for the current system locale,
// cached after first call. Set FINDMY_LANG to override detection.
func GetAppStrings() *AppStrings {
	appStringsOnce.Do(func() {
		appStrings = resolveAppStrings()
	})
	return appStrings
}

func resolveAppStrings() *AppStrings {
	lang := os.Getenv("FINDMY_LANG")
	if lang == "" {
		lang = detectSystemLanguage()
	}

	s := lookupStrings(lang)

	// Window owner: prefer runtime detection via Spotlight metadata, which
	// returns the correct localized name for any locale without a map.
	if owner := detectWindowOwner(); owner != "" {
		s.WindowOwner = owner
	}

	return s
}

// detectWindowOwner queries Spotlight for the Finder display name of FindMy.app.
// This respects the system locale and works for all 41 languages without a map.
func detectWindowOwner() string {
	out, err := exec.Command("mdls", "-raw", "-name", "kMDItemDisplayName",
		"/System/Applications/FindMy.app").Output()
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(string(out))
	if name == "" || name == "(null)" {
		return ""
	}
	return name
}

// detectSystemLanguage reads the primary language from macOS user defaults.
// Returns tags like "fr-FR", "en-US", "de-DE".
func detectSystemLanguage() string {
	out, err := exec.Command("defaults", "read", "-g", "AppleLanguages").Output()
	if err != nil {
		return "en"
	}
	// Output looks like: (\n    "fr-FR"\n)
	s := string(out)
	start := strings.Index(s, `"`)
	if start < 0 {
		return "en"
	}
	end := strings.Index(s[start+1:], `"`)
	if end < 0 {
		return "en"
	}
	return s[start+1 : start+1+end]
}

func lookupStrings(lang string) *AppStrings {
	if s, ok := localeTable[lang]; ok {
		return s.clone()
	}
	normalized := strings.ReplaceAll(lang, "-", "_")
	if s, ok := localeTable[normalized]; ok {
		return s.clone()
	}
	if strings.HasPrefix(normalized, "zh_Hant") {
		return localeTable["zh_TW"].clone()
	}
	if strings.HasPrefix(normalized, "zh_Hans") {
		return localeTable["zh_CN"].clone()
	}
	// Try base language: "fr-FR" → "fr".
	if idx := strings.IndexAny(lang, "-_"); idx > 0 {
		base := lang[:idx]
		if s, ok := localeTable[base]; ok {
			return s.clone()
		}
	}
	return localeTable["en"].clone()
}

func (s *AppStrings) clone() *AppStrings {
	c := *s
	c.TimeSuffixes = append([]string{}, s.TimeSuffixes...)
	return &c
}

// SkipWords returns the set of OCR text to ignore when parsing the sidebar.
func (s *AppStrings) SkipWords() map[string]bool {
	skip := map[string]bool{
		// Universal UI elements
		"FaceTime": true, "+": true, "3D": true, "N": true,
		// Current locale
		s.PeopleTab:   true,
		s.DevicesTab:  true,
		s.ItemsTab:    true,
		s.SearchLabel: true,
	}
	// Always include English (the OCR sometimes picks up both)
	for _, w := range []string{"People", "Devices", "Items", "Search"} {
		skip[w] = true
	}
	return skip
}

// --- Time suffix patterns ---

var defaultTimeSuffixes = []string{
	"min. ago", "min ago", "hr. ago", "hr ago",
	"sec. ago", "sec ago", "day ago", "days ago",
	"week ago", "weeks ago", "month ago", "months ago",
	"year ago", "years ago", "ago",
}

var frTimeSuffixes = []string{
	"il y a", "maintenant", "à l'instant",
	// Wrapped continuations: "Paris • il y a\n5 min" → "5 min"
	"min", "min.", "h", "j", "jour", "jours",
	"sem.", "semaine", "semaines", "mois", "an", "ans",
}

// --- Locale table ---
//
// All 41 languages extracted from macOS bundle .loctable files:
//   View menu   → AppKit MenuCommands.loctable
//   People/Devices/Items tabs → FindMy Localizable.loctable / Localizable-HAWKEYE.loctable
//   Search      → FindMy Localizable.loctable (SEARCH_BAR_PLACEHOLDER_ALTERNATIVE)
//   WindowOwner → FindMy InfoPlist.loctable (fallback; runtime mdls detection preferred)
//
// TimeSuffixes: only en and fr have specific patterns. Other locales inherit
// defaultTimeSuffixes which is sufficient for the wrapped-line merging heuristic.

var localeTable = map[string]*AppStrings{
	"ar":     {WindowOwner: "تحديد الموقع", ViewMenu: "عرض", PeopleTab: "الأشخاص", DevicesTab: "الأجهزة", ItemsTab: "الأغراض", SearchLabel: "بحث", TimeSuffixes: defaultTimeSuffixes},
	"ca":     {WindowOwner: "Cerca", ViewMenu: "Mostra", PeopleTab: "Persones", DevicesTab: "Dispositius", ItemsTab: "Objectes", SearchLabel: "Cerca", TimeSuffixes: defaultTimeSuffixes},
	"cs":     {WindowOwner: "Najít", ViewMenu: "Zobrazení", PeopleTab: "Lidé", DevicesTab: "Zařízení", ItemsTab: "Předměty", SearchLabel: "Hledat", TimeSuffixes: defaultTimeSuffixes},
	"da":     {WindowOwner: "Find", ViewMenu: "Oversigt", PeopleTab: "Personer", DevicesTab: "Enheder", ItemsTab: "Genstande", SearchLabel: "Søg", TimeSuffixes: defaultTimeSuffixes},
	"de":     {WindowOwner: "Wo ist?", ViewMenu: "Darstellung", PeopleTab: "Personen", DevicesTab: "Geräte", ItemsTab: "Objekte", SearchLabel: "Suchen", TimeSuffixes: defaultTimeSuffixes},
	"el":     {WindowOwner: "Εύρεση", ViewMenu: "Προβολή", PeopleTab: "Άτομα", DevicesTab: "Συσκευές", ItemsTab: "Αντικείμενα", SearchLabel: "Αναζήτηση", TimeSuffixes: defaultTimeSuffixes},
	"en":     {WindowOwner: "Find My", ViewMenu: "View", PeopleTab: "People", DevicesTab: "Devices", ItemsTab: "Items", SearchLabel: "Search", TimeSuffixes: defaultTimeSuffixes},
	"en_AU":  {WindowOwner: "Find My", ViewMenu: "View", PeopleTab: "People", DevicesTab: "Devices", ItemsTab: "Items", SearchLabel: "Search", TimeSuffixes: defaultTimeSuffixes},
	"en_GB":  {WindowOwner: "Find My", ViewMenu: "View", PeopleTab: "People", DevicesTab: "Devices", ItemsTab: "Items", SearchLabel: "Search", TimeSuffixes: defaultTimeSuffixes},
	"es":     {WindowOwner: "Buscar", ViewMenu: "Visualización", PeopleTab: "Personas", DevicesTab: "Dispositivos", ItemsTab: "Objetos", SearchLabel: "Buscar", TimeSuffixes: defaultTimeSuffixes},
	"es_419": {WindowOwner: "Encontrar", ViewMenu: "Visualización", PeopleTab: "Personas", DevicesTab: "Dispositivos", ItemsTab: "Artículos", SearchLabel: "Buscar", TimeSuffixes: defaultTimeSuffixes},
	"es_US":  {WindowOwner: "Encontrar", ViewMenu: "Visualización", PeopleTab: "Personas", DevicesTab: "Dispositivos", ItemsTab: "Artículos", SearchLabel: "Buscar", TimeSuffixes: defaultTimeSuffixes},
	"fi":     {WindowOwner: "Etsi", ViewMenu: "Näytä", PeopleTab: "Käyttäjät", DevicesTab: "Laitteet", ItemsTab: "Esineet", SearchLabel: "Etsi", TimeSuffixes: defaultTimeSuffixes},
	"fr":     {WindowOwner: "Localiser", ViewMenu: "Présentation", PeopleTab: "Personnes", DevicesTab: "Appareils", ItemsTab: "Objets", SearchLabel: "Rechercher", TimeSuffixes: append(append([]string{}, frTimeSuffixes...), defaultTimeSuffixes...)},
	"fr_CA":  {WindowOwner: "Localiser", ViewMenu: "Présentation", PeopleTab: "Personnes", DevicesTab: "Appareils", ItemsTab: "Objets", SearchLabel: "Rechercher", TimeSuffixes: append(append([]string{}, frTimeSuffixes...), defaultTimeSuffixes...)},
	"he":     {WindowOwner: "איתור", ViewMenu: "תצוגה", PeopleTab: "אנשים", DevicesTab: "מכשירים", ItemsTab: "פריטים", SearchLabel: "חיפוש", TimeSuffixes: defaultTimeSuffixes},
	"hi":     {WindowOwner: "Find My", ViewMenu: "दृश्य", PeopleTab: "लोग", DevicesTab: "डिवाइस", ItemsTab: "आइटम", SearchLabel: "खोजें", TimeSuffixes: defaultTimeSuffixes},
	"hr":     {WindowOwner: "Pronalaženje", ViewMenu: "Prikaz", PeopleTab: "Osobe", DevicesTab: "Uređaji", ItemsTab: "Predmeti", SearchLabel: "Pretraga", TimeSuffixes: defaultTimeSuffixes},
	"hu":     {WindowOwner: "Lokátor", ViewMenu: "Nézet", PeopleTab: "Személyek", DevicesTab: "Eszközök", ItemsTab: "Tárgyak", SearchLabel: "Keresés", TimeSuffixes: defaultTimeSuffixes},
	"id":     {WindowOwner: "Lacak", ViewMenu: "Lihat", PeopleTab: "Orang", DevicesTab: "Perangkat", ItemsTab: "Barang", SearchLabel: "Cari", TimeSuffixes: defaultTimeSuffixes},
	"it":     {WindowOwner: "Dov'è", ViewMenu: "Vista", PeopleTab: "Persone", DevicesTab: "Dispositivi", ItemsTab: "Oggetti", SearchLabel: "Cerca", TimeSuffixes: defaultTimeSuffixes},
	"ja":     {WindowOwner: "探す", ViewMenu: "表示", PeopleTab: "人を探す", DevicesTab: "デバイスを探す", ItemsTab: "持ち物を探す", SearchLabel: "検索", TimeSuffixes: defaultTimeSuffixes},
	"ko":     {WindowOwner: "나의 찾기", ViewMenu: "보기", PeopleTab: "사람", DevicesTab: "기기", ItemsTab: "물품", SearchLabel: "검색", TimeSuffixes: defaultTimeSuffixes},
	"ms":     {WindowOwner: "Cari", ViewMenu: "Paparan", PeopleTab: "Orang", DevicesTab: "Peranti", ItemsTab: "Item", SearchLabel: "Cari", TimeSuffixes: defaultTimeSuffixes},
	"nl":     {WindowOwner: "Zoek mijn", ViewMenu: "Weergave", PeopleTab: "Personen", DevicesTab: "Apparaten", ItemsTab: "Objecten", SearchLabel: "Zoek", TimeSuffixes: defaultTimeSuffixes},
	"no":     {WindowOwner: "Hvor er", ViewMenu: "Vis", PeopleTab: "Personer", DevicesTab: "Enheter", ItemsTab: "Objekter", SearchLabel: "Søk", TimeSuffixes: defaultTimeSuffixes},
	"pl":     {WindowOwner: "Znajdź", ViewMenu: "Widok", PeopleTab: "Osoby", DevicesTab: "Urządzenia", ItemsTab: "Przedmioty", SearchLabel: "Szukaj", TimeSuffixes: defaultTimeSuffixes},
	"pt":     {WindowOwner: "Buscar", ViewMenu: "Visualizar", PeopleTab: "Pessoas", DevicesTab: "Dispositivos", ItemsTab: "Itens", SearchLabel: "Buscar", TimeSuffixes: defaultTimeSuffixes},
	"pt_PT":  {WindowOwner: "Encontrar", ViewMenu: "Visualização", PeopleTab: "Pessoas", DevicesTab: "Dispositivos", ItemsTab: "Objetos", SearchLabel: "Procurar", TimeSuffixes: defaultTimeSuffixes},
	"ro":     {WindowOwner: "Găsire", ViewMenu: "Vizualizare", PeopleTab: "Persoane", DevicesTab: "Dispozitive", ItemsTab: "Articole", SearchLabel: "Căutare", TimeSuffixes: defaultTimeSuffixes},
	"ru":     {WindowOwner: "Локатор", ViewMenu: "Вид", PeopleTab: "Люди", DevicesTab: "Устройства", ItemsTab: "Вещи", SearchLabel: "Поиск", TimeSuffixes: defaultTimeSuffixes},
	"sk":     {WindowOwner: "Nájsť", ViewMenu: "Zobraziť", PeopleTab: "Ľudia", DevicesTab: "Zariadenia", ItemsTab: "Predmety", SearchLabel: "Vyhľadať", TimeSuffixes: defaultTimeSuffixes},
	"sl":     {WindowOwner: "Najdi", ViewMenu: "Prikaz", PeopleTab: "Osebe", DevicesTab: "Naprave", ItemsTab: "Predmeti", SearchLabel: "Iskanje", TimeSuffixes: defaultTimeSuffixes},
	"sv":     {WindowOwner: "Hitta", ViewMenu: "Innehåll", PeopleTab: "Personer", DevicesTab: "Enheter", ItemsTab: "Föremål", SearchLabel: "Sök", TimeSuffixes: defaultTimeSuffixes},
	"th":     {WindowOwner: "ค้นหาของฉัน", ViewMenu: "มุมมอง", PeopleTab: "ผู้คน", DevicesTab: "อุปกรณ์", ItemsTab: "สิ่งของ", SearchLabel: "ค้นหา", TimeSuffixes: defaultTimeSuffixes},
	"tr":     {WindowOwner: "Bul", ViewMenu: "Görüntü", PeopleTab: "Kişiler", DevicesTab: "Aygıtlar", ItemsTab: "Nesneler", SearchLabel: "Arayın", TimeSuffixes: defaultTimeSuffixes},
	"uk":     {WindowOwner: "Локатор", ViewMenu: "Перегляд", PeopleTab: "Люди", DevicesTab: "Пристрої", ItemsTab: "Речі", SearchLabel: "Шукати", TimeSuffixes: defaultTimeSuffixes},
	"vi":     {WindowOwner: "Tìm", ViewMenu: "Xem", PeopleTab: "Người", DevicesTab: "Thiết bị", ItemsTab: "Vật dụng", SearchLabel: "Tìm kiếm", TimeSuffixes: defaultTimeSuffixes},
	"zh_CN":  {WindowOwner: "查找", ViewMenu: "显示", PeopleTab: "联系人", DevicesTab: "设备", ItemsTab: "物品", SearchLabel: "搜索", TimeSuffixes: defaultTimeSuffixes},
	"zh_HK":  {WindowOwner: "尋找", ViewMenu: "顯示方式", PeopleTab: "聯絡人", DevicesTab: "裝置", ItemsTab: "物品", SearchLabel: "搜尋", TimeSuffixes: defaultTimeSuffixes},
	"zh_TW":  {WindowOwner: "尋找", ViewMenu: "顯示方式", PeopleTab: "聯絡人", DevicesTab: "裝置", ItemsTab: "物品", SearchLabel: "搜尋", TimeSuffixes: defaultTimeSuffixes},
}
