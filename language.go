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

	// ── Download tab ───────────────────────────────────────────────
	"dl.title": {
		state.LangEnglish: "Music Download",
		state.LangTurkish: "Müzik İndirme",
	},
	"dl.spotify_url": {
		state.LangEnglish: "Spotify URL (track/playlist):",
		state.LangTurkish: "Spotify URL (parça/çalma listesi):",
	},
	"dl.youtube_url": {
		state.LangEnglish: "YouTube URL:",
		state.LangTurkish: "YouTube URL:",
	},
	"dl.btn_playlist": {
		state.LangEnglish: "+ Playlist",
		state.LangTurkish: "+ Çalma Listesi",
	},
	"dl.btn_music": {
		state.LangEnglish: "+ Music",
		state.LangTurkish: "+ Müzik",
	},
	"dl.btn_download": {
		state.LangEnglish: "v Download",
		state.LangTurkish: "v İndir",
	},
	"dl.no_playlists": {
		state.LangEnglish: "(no playlists)",
		state.LangTurkish: "(çalma listesi yok)",
	},
	"dl.enter_url": {
		state.LangEnglish: "Enter a URL first",
		state.LangTurkish: "Önce bir URL girin",
	},
	"dl.invalid_url": {
		state.LangEnglish: "Invalid URL",
		state.LangTurkish: "Geçersiz URL",
	},
	"dl.session_ok": {
		state.LangEnglish: "OK",
		state.LangTurkish: "Başarılı",
	},
	"dl.session_failed": {
		state.LangEnglish: "failed",
		state.LangTurkish: "başarısız",
	},
	"dl.session_summary": {
		state.LangEnglish: "Session: %d %s, %d %s",
		state.LangTurkish: "Oturum: %d %s, %d %s",
	},
	"dl.downloading": {
		state.LangEnglish: "downloading",
		state.LangTurkish: "indiriliyor",
	},
	"dl.complete": {
		state.LangEnglish: "Download complete",
		state.LangTurkish: "İndirme tamamlandı",
	},
	"dl.error": {
		state.LangEnglish: "Error",
		state.LangTurkish: "Hata",
	},
	"dl.no_logs": {
		state.LangEnglish: "No logs",
		state.LangTurkish: "Kayıt yok",
	},
	"dl.downloaded_n": {
		state.LangEnglish: "Downloaded %d song(s)",
		state.LangTurkish: "%d parça indirildi",
	},
	"dl.errors_n": {
		state.LangEnglish: "completed with %d error(s)",
		state.LangTurkish: "%d hata ile tamamlandı",
	},
}
