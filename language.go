package main

import "MusicLeCLI/state"

// Tr returns the translation for key in the current language.
func Tr(key string) string {
	m, ok := allTranslations[key]
	if !ok {
		return key
	}
	lang := state.Current.Language
	if t, ok := m[lang]; ok {
		return t
	}
	if t, ok := m[state.LangEnglish]; ok {
		return t
	}
	return key
}

var allTranslations = map[string]map[state.Language]string{

	// ── Language endonyms (shown in language picker) ──────────────
	"lang.en": {
		state.LangEnglish: "English",
		state.LangTurkish: "İngilizce",
	},
	"lang.tr": {
		state.LangEnglish: "Turkish",
		state.LangTurkish: "Türkçe",
	},
	"lang.ru": {
		state.LangEnglish: "Russian",
		state.LangTurkish: "Rusça",
	},
	"lang.es": {
		state.LangEnglish: "Spanish",
		state.LangTurkish: "İspanyolca",
	},
	"lang.it": {
		state.LangEnglish: "Italian",
		state.LangTurkish: "İtalyanca",
	},
	"lang.ar": {
		state.LangEnglish: "Arabic",
		state.LangTurkish: "Arapça",
	},
	"lang.zh": {
		state.LangEnglish: "Chinese",
		state.LangTurkish: "Çince",
	},
	"lang.fr": {
		state.LangEnglish: "French",
		state.LangTurkish: "Fransızca",
	},

	// ── Navigation ────────────────────────────────────────────────
	"nav.home": {
		state.LangEnglish: "Home",
		state.LangTurkish: "Ana Sayfa",
	},
	"nav.downloads": {
		state.LangEnglish: "Downloads",
		state.LangTurkish: "İndirilenler",
	},
	"nav.profile": {
		state.LangEnglish: "Profile",
		state.LangTurkish: "Profil",
	},
	"nav.playlist": {
		state.LangEnglish: "Playlist",
		state.LangTurkish: "Çalma Listesi",
	},
	"nav.general": {
		state.LangEnglish: "General",
		state.LangTurkish: "Genel",
	},

	// ── Settings / General tabs ───────────────────────────────────
	"settings.title": {
		state.LangEnglish: "General Settings",
		state.LangTurkish: "Genel Ayarlar",
	},
	"tab.theme": {
		state.LangEnglish: "Theme",
		state.LangTurkish: "Tema",
	},
	"tab.language": {
		state.LangEnglish: "Language",
		state.LangTurkish: "Dil",
	},
	"tab.sound": {
		state.LangEnglish: "Sound",
		state.LangTurkish: "Ses",
	},
	"tab.extras": {
		state.LangEnglish: "Extras",
		state.LangTurkish: "Ekstralar",
	},
	"tab.policies": {
		state.LangEnglish: "Policies",
		state.LangTurkish: "Politikalar",
	},
	"tab.about": {
		state.LangEnglish: "About",
		state.LangTurkish: "Hakkında",
	},
	"settings.f3_hint": {
		state.LangEnglish: "[F3] switch tab",
		state.LangTurkish: "[F3] sekme değiştir",
	},
	"settings.select_hint": {
		state.LangEnglish: "[↑↓] Change  [Enter] Apply  [Tab] Leave",
		state.LangTurkish: "[↑↓] Değiştir  [Enter] Uygula  [Tab] Çık",
	},

	// ── Common ────────────────────────────────────────────────────
	"common.coming_soon": {
		state.LangEnglish: "Coming soon",
		state.LangTurkish: "Yakında",
	},
	"common.no_playlist": {
		state.LangEnglish: "No playlist selected",
		state.LangTurkish: "Çalma listesi seçilmedi",
	},
}
