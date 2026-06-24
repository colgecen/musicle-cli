package test

import (
	"os"
	"path/filepath"
	"testing"

	"MusicLeCLI/state"
)

func makeTempState(t *testing.T) (string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "musicle_test_*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	return dir, func() { os.RemoveAll(dir) }
}

func writeSongList(t *testing.T, path string, lines []string) {
	t.Helper()
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write song_list: %v", err)
	}
}

func TestT_Language(t *testing.T) {
	if got := state.T(state.LangEnglish, "hello", "merhaba"); got != "hello" {
		t.Errorf("LangEnglish: got %q, want %q", got, "hello")
	}
	if got := state.T(state.LangTurkish, "hello", "merhaba"); got != "merhaba" {
		t.Errorf("LangTurkish: got %q, want %q", got, "merhaba")
	}
}

func TestReadSongs(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	listPath := filepath.Join(dir, "song_list.txt")
	writeSongList(t, listPath, []string{
		"song1.mp3|Title One|Artist A|2024-01-01|03:30",
		"song2.mp3|Title Two|Artist B|2024-01-02|04:00",
	})

	songs, err := state.ReadSongs(listPath)
	if err != nil {
		t.Fatalf("ReadSongs: %v", err)
	}
	if len(songs) != 2 {
		t.Fatalf("got %d songs, want 2", len(songs))
	}
	if songs[0].Filename != "song1.mp3" || songs[0].Title != "Title One" {
		t.Errorf("song0: %+v", songs[0])
	}
	if songs[1].Filename != "song2.mp3" || songs[1].Artist != "Artist B" {
		t.Errorf("song1: %+v", songs[1])
	}
}

func TestReadSongs_MissingFile(t *testing.T) {
	_, err := state.ReadSongs("nonexistent.txt")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadSongs_EmptyFile(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	listPath := filepath.Join(dir, "song_list.txt")
	os.WriteFile(listPath, []byte(""), 0644)

	songs, err := state.ReadSongs(listPath)
	if err != nil {
		t.Fatalf("ReadSongs: %v", err)
	}
	if len(songs) != 0 {
		t.Fatalf("expected 0 songs, got %d", len(songs))
	}
}

func TestReadSongs_SkipsBadLines(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	listPath := filepath.Join(dir, "song_list.txt")
	writeSongList(t, listPath, []string{
		"valid.mp3|Title|Artist|2024-01-01|03:30",
		"bad_line_no_pipes",
		"also_bad|only|three",
		"another.mp3|Another|Artist2|2024-01-02|04:00",
	})

	songs, err := state.ReadSongs(listPath)
	if err != nil {
		t.Fatalf("ReadSongs: %v", err)
	}
	if len(songs) != 2 {
		t.Fatalf("expected 2 valid songs, got %d", len(songs))
	}
}

func TestWriteSongs(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	listPath := filepath.Join(dir, "song_list.txt")
	songs := []state.Song{
		{Filename: "a.mp3", Title: "A", Artist: "Art", DateAdded: "2024-01-01", Duration: "03:00"},
		{Filename: "b.mp3", Title: "B", Artist: "Art2", DateAdded: "2024-01-02", Duration: "04:00"},
	}

	if err := state.WriteSongs(listPath, songs); err != nil {
		t.Fatalf("WriteSongs: %v", err)
	}

	read, err := state.ReadSongs(listPath)
	if err != nil {
		t.Fatalf("ReadSongs after write: %v", err)
	}
	if len(read) != 2 {
		t.Fatalf("got %d songs, want 2", len(read))
	}
}

func TestWriteSongs_Empty(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	listPath := filepath.Join(dir, "song_list.txt")
	if err := state.WriteSongs(listPath, nil); err != nil {
		t.Fatalf("WriteSongs(nil): %v", err)
	}
	songs, _ := state.ReadSongs(listPath)
	if len(songs) != 0 {
		t.Fatal("expected empty file")
	}
}

func TestRemoveSong(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	listPath := filepath.Join(dir, "song_list.txt")
	writeSongList(t, listPath, []string{
		"s1.mp3|One|A1|2024-01-01|03:00",
		"s2.mp3|Two|A2|2024-01-02|04:00",
		"s3.mp3|Three|A3|2024-01-03|05:00",
	})

	if err := state.RemoveSong(listPath, "s2.mp3"); err != nil {
		t.Fatalf("RemoveSong: %v", err)
	}

	songs, _ := state.ReadSongs(listPath)
	if len(songs) != 2 {
		t.Fatalf("expected 2 songs after remove, got %d", len(songs))
	}
	if songs[0].Filename != "s1.mp3" || songs[1].Filename != "s3.mp3" {
		t.Errorf("remaining songs wrong: %+v", songs)
	}
}

func TestRemoveSong_NotFound(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	listPath := filepath.Join(dir, "song_list.txt")
	writeSongList(t, listPath, []string{
		"s1.mp3|One|A1|2024-01-01|03:00",
	})

	err := state.RemoveSong(listPath, "nonexistent.mp3")
	if err == nil {
		t.Fatal("expected error for nonexistent song")
	}
}

func TestRemoveSong_EmptyFile(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	listPath := filepath.Join(dir, "song_list.txt")
	os.WriteFile(listPath, []byte(""), 0644)

	err := state.RemoveSong(listPath, "any.mp3")
	if err == nil {
		t.Fatal("expected error when removing from empty list")
	}
}

func TestUpdateSong(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	listPath := filepath.Join(dir, "song_list.txt")
	writeSongList(t, listPath, []string{
		"s1.mp3|Old Title|Old Artist|2024-01-01|03:00",
	})

	if err := state.UpdateSong(listPath, "s1.mp3", "New Title", "New Artist", "04:30"); err != nil {
		t.Fatalf("UpdateSong: %v", err)
	}

	songs, _ := state.ReadSongs(listPath)
	if len(songs) != 1 {
		t.Fatalf("expected 1 song, got %d", len(songs))
	}
	if songs[0].Title != "New Title" || songs[0].Artist != "New Artist" || songs[0].Duration != "04:30" {
		t.Errorf("update fields wrong: %+v", songs[0])
	}
}

func TestUpdateSong_Partial(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	listPath := filepath.Join(dir, "song_list.txt")
	writeSongList(t, listPath, []string{
		"s1.mp3|Old Title|Old Artist|2024-01-01|03:00",
	})

	if err := state.UpdateSong(listPath, "s1.mp3", "Only Title", "", ""); err != nil {
		t.Fatalf("UpdateSong partial: %v", err)
	}

	songs, _ := state.ReadSongs(listPath)
	if songs[0].Title != "Only Title" || songs[0].Artist != "Old Artist" || songs[0].Duration != "03:00" {
		t.Errorf("partial update wrong: %+v", songs[0])
	}
}

func TestUpdateSong_NotFound(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	listPath := filepath.Join(dir, "song_list.txt")
	writeSongList(t, listPath, []string{
		"s1.mp3|Title|Artist|2024-01-01|03:00",
	})

	err := state.UpdateSong(listPath, "nope.mp3", "New", "New", "00:00")
	if err == nil {
		t.Fatal("expected error for nonexistent song")
	}
}

func TestAppendSong(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	listPath := filepath.Join(dir, "song_list.txt")
	if err := state.AppendSong(listPath, "new.mp3", "New Song", "New Artist", "05:00"); err != nil {
		t.Fatalf("AppendSong: %v", err)
	}

	songs, _ := state.ReadSongs(listPath)
	if len(songs) != 1 {
		t.Fatalf("expected 1 song, got %d", len(songs))
	}
	if songs[0].Filename != "new.mp3" || songs[0].Title != "New Song" {
		t.Errorf("append result: %+v", songs[0])
	}
}

func TestAppendSong_Multiple(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	listPath := filepath.Join(dir, "song_list.txt")
	state.AppendSong(listPath, "a.mp3", "A", "Art", "01:00")
	state.AppendSong(listPath, "b.mp3", "B", "Art", "02:00")

	songs, _ := state.ReadSongs(listPath)
	if len(songs) != 2 {
		t.Fatalf("expected 2 songs, got %d", len(songs))
	}
}

func TestLoadConfig(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	cfgDir := filepath.Join(dir, "config")
	os.MkdirAll(cfgDir, 0755)
	cfgPath := filepath.Join(cfgDir, "config.json")
	cfgContent := `{
		"root_dir": "C:\\music",
		"language": "tr",
		"last_user": "testuser",
		"theme": "blue"
	}`
	os.WriteFile(cfgPath, []byte(cfgContent), 0644)

	app := &state.AppState{ConfigDir: cfgDir}
	if err := app.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if app.RootDir != "C:\\music" {
		t.Errorf("RootDir = %q, want %q", app.RootDir, "C:\\music")
	}
	if app.Language != state.LangTurkish {
		t.Errorf("Language = %q, want %q", app.Language, state.LangTurkish)
	}
	if app.Theme != "blue" {
		t.Errorf("Theme = %q, want %q", app.Theme, "blue")
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	app := &state.AppState{ConfigDir: filepath.Join(dir, "nonexistent")}
	err := app.LoadConfig()
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestSaveConfig(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	app := &state.AppState{
		ConfigDir: filepath.Join(dir, "config"),
		RootDir:   "/music",
		Language:  state.LangEnglish,
		Theme:     "purple",
	}

	if err := app.SaveConfig(); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	// Reload
	app2 := &state.AppState{ConfigDir: app.ConfigDir}
	if err := app2.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig after save: %v", err)
	}
	if app2.RootDir != "/music" || app2.Theme != "purple" {
		t.Errorf("reloaded: RootDir=%q Theme=%q", app2.RootDir, app2.Theme)
	}
}

func TestSaveConfig_WithProfile(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	app := &state.AppState{
		ConfigDir: filepath.Join(dir, "config"),
		CurrentProfile: &state.Profile{FolderName: "myprofile"},
	}

	if err := app.SaveConfig(); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	app2 := &state.AppState{ConfigDir: app.ConfigDir}
	app2.LoadConfig()
	// LastUser is not directly accessible via LoadConfig, but config is saved
}

func TestSaveConfig_EmptyThemeDefaults(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	cfgDir := filepath.Join(dir, "config")
	os.MkdirAll(cfgDir, 0755)
	cfgContent := `{"theme": ""}`
	os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(cfgContent), 0644)

	app := &state.AppState{ConfigDir: cfgDir}
	app.LoadConfig()
	if app.Theme != "green" {
		t.Errorf("empty theme should default to green, got %q", app.Theme)
	}
}

func TestInitializeBaseDirs(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	root := filepath.Join(dir, "music")
	app := &state.AppState{}
	if err := app.InitializeBaseDirs(root); err != nil {
		t.Fatalf("InitializeBaseDirs: %v", err)
	}
	if app.RootDir != root {
		t.Errorf("RootDir = %q, want %q", app.RootDir, root)
	}
	if _, err := os.Stat(app.ProfilesDir()); os.IsNotExist(err) {
		t.Errorf("ProfilesDir was not created: %v", err)
	}
}

func TestProfilesDir(t *testing.T) {
	app := &state.AppState{RootDir: "/base"}
	want := filepath.Join("/base", "profiles")
	if got := app.ProfilesDir(); got != want {
		t.Errorf("ProfilesDir = %q, want %q", got, want)
	}
}

func TestCreateProfileStructure(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	root := filepath.Join(dir, "music")
	app := &state.AppState{RootDir: root}
	app.InitializeBaseDirs(root)

	if err := app.CreateProfileStructure("testuser", "Test User", "A bio", "", state.LangTurkish); err != nil {
		t.Fatalf("CreateProfileStructure: %v", err)
	}

	profDir := filepath.Join(root, "profiles", "testuser")
	if _, err := os.Stat(profDir); os.IsNotExist(err) {
		t.Fatal("profile dir was not created")
	}

	checkFile := func(name, expected string) {
		data, err := os.ReadFile(filepath.Join(profDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(data) != expected {
			t.Errorf("%s = %q, want %q", name, string(data), expected)
		}
	}
	checkFile("name.txt", "Test User")
	checkFile("bio.txt", "A bio")
	checkFile("lang.txt", "tr")

	// Verify subdirs exist
	for _, d := range []string{"avatar", "playlists"} {
		fi, err := os.Stat(filepath.Join(profDir, d))
		if err != nil || !fi.IsDir() {
			t.Errorf("subdir %s missing or not a dir", d)
		}
	}
}

func TestCreatePlaylistStructure(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	root := filepath.Join(dir, "music")
	app := &state.AppState{RootDir: root}
	app.InitializeBaseDirs(root)
	app.CreateProfileStructure("user", "User", "", "", state.LangEnglish)

	if err := app.CreatePlaylistStructure("user", "mypl", "My Playlist", "My bio", ""); err != nil {
		t.Fatalf("CreatePlaylistStructure: %v", err)
	}

	plDir := filepath.Join(root, "profiles", "user", "playlists", "mypl")
	if _, err := os.Stat(plDir); os.IsNotExist(err) {
		t.Fatal("playlist dir was not created")
	}

	data, _ := os.ReadFile(filepath.Join(plDir, "playlist_name.txt"))
	if string(data) != "My Playlist" {
		t.Errorf("playlist_name.txt = %q, want %q", string(data), "My Playlist")
	}

	data, _ = os.ReadFile(filepath.Join(plDir, "playlist_bio.txt"))
	if string(data) != "My bio" {
		t.Errorf("playlist_bio.txt = %q, want %q", string(data), "My bio")
	}
}

func TestSaveProfileMeta(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	root := filepath.Join(dir, "music")
	app := &state.AppState{RootDir: root}
	app.InitializeBaseDirs(root)
	app.CreateProfileStructure("u", "Old", "", "", state.LangEnglish)

	app.SaveProfileMeta("u", "New Name", "New bio")
	data, _ := os.ReadFile(filepath.Join(root, "profiles", "u", "name.txt"))
	if string(data) != "New Name" {
		t.Errorf("name = %q", string(data))
	}
	data, _ = os.ReadFile(filepath.Join(root, "profiles", "u", "bio.txt"))
	if string(data) != "New bio" {
		t.Errorf("bio = %q", string(data))
	}
}

func TestSavePlaylistMeta(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	root := filepath.Join(dir, "music")
	app := &state.AppState{RootDir: root}
	app.InitializeBaseDirs(root)
	app.CreateProfileStructure("u", "U", "", "", state.LangEnglish)
	app.CreatePlaylistStructure("u", "pl", "Old", "Old bio", "")

	app.SavePlaylistMeta("u", "pl", "New PL", "New bio")
	plDir := filepath.Join(root, "profiles", "u", "playlists", "pl")
	data, _ := os.ReadFile(filepath.Join(plDir, "playlist_name.txt"))
	if string(data) != "New PL" {
		t.Errorf("name = %q", string(data))
	}
	data, _ = os.ReadFile(filepath.Join(plDir, "playlist_bio.txt"))
	if string(data) != "New bio" {
		t.Errorf("bio = %q", string(data))
	}
}

func TestDeletePlaylist(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	root := filepath.Join(dir, "music")
	app := &state.AppState{RootDir: root}
	app.InitializeBaseDirs(root)
	app.CreateProfileStructure("u", "U", "", "", state.LangEnglish)
	app.CreatePlaylistStructure("u", "pl", "PL", "B", "")

	if err := app.DeletePlaylist("u", "pl"); err != nil {
		t.Fatalf("DeletePlaylist: %v", err)
	}

	plDir := filepath.Join(root, "profiles", "u", "playlists", "pl")
	if _, err := os.Stat(plDir); !os.IsNotExist(err) {
		t.Errorf("playlist dir should be deleted: %v", err)
	}
}

func TestSongListPath(t *testing.T) {
	app := &state.AppState{RootDir: "/base"}
	got := app.SongListPath("user1", "mypl")
	want := filepath.Join("/base", "profiles", "user1", "playlists", "mypl", "song_list.txt")
	if got != want {
		t.Errorf("SongListPath = %q, want %q", got, want)
	}
}

func TestPlaylistDir(t *testing.T) {
	app := &state.AppState{RootDir: "/base"}
	got := app.PlaylistDir("user1", "mypl")
	want := filepath.Join("/base", "profiles", "user1", "playlists", "mypl")
	if got != want {
		t.Errorf("PlaylistDir = %q, want %q", got, want)
	}
}

func TestLoadPlaylist(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	plDir := filepath.Join(dir, "mypl")
	os.MkdirAll(plDir, 0755)
	os.WriteFile(filepath.Join(plDir, "playlist_name.txt"), []byte("Test PL"), 0644)
	os.WriteFile(filepath.Join(plDir, "playlist_bio.txt"), []byte("Test bio"), 0644)
	writeSongList(t, filepath.Join(plDir, "song_list.txt"), []string{
		"s.mp3|Song|Artist|2024-01-01|03:00",
	})

	pl, err := state.LoadPlaylist(plDir, "mypl")
	if err != nil {
		t.Fatalf("LoadPlaylist: %v", err)
	}
	if pl.Name != "Test PL" {
		t.Errorf("Name = %q", pl.Name)
	}
	if pl.Bio != "Test bio" {
		t.Errorf("Bio = %q", pl.Bio)
	}
	if len(pl.Songs) != 1 {
		t.Fatalf("expected 1 song, got %d", len(pl.Songs))
	}
}

func TestLoadPlaylist_FallbackName(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	plDir := filepath.Join(dir, "mypl")
	os.MkdirAll(plDir, 0755)

	pl, err := state.LoadPlaylist(plDir, "mypl")
	if err != nil {
		t.Fatalf("LoadPlaylist: %v", err)
	}
	if pl.Name != "mypl" {
		t.Errorf("Name should fall back to folder name, got %q", pl.Name)
	}
}

func TestCopyFile(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "sub", "dst.txt")
	os.WriteFile(src, []byte("hello world"), 0644)

	if err := state.CopyFile(src, dst); err != nil {
		t.Fatalf("CopyFile: %v", err)
	}

	data, _ := os.ReadFile(dst)
	if string(data) != "hello world" {
		t.Errorf("copied content = %q, want %q", string(data), "hello world")
	}
}

func TestCopyFile_SrcNotFound(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	err := state.CopyFile(filepath.Join(dir, "nonexistent"), filepath.Join(dir, "out.txt"))
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
}

func TestScanProfiles(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	root := filepath.Join(dir, "music")
	app := &state.AppState{RootDir: root}
	app.InitializeBaseDirs(root)

	app.CreateProfileStructure("u1", "User One", "Bio1", "", state.LangEnglish)
	app.CreateProfileStructure("u2", "User Two", "Bio2", "", state.LangTurkish)

	if err := app.ScanProfiles(); err != nil {
		t.Fatalf("ScanProfiles: %v", err)
	}
	if len(app.Profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(app.Profiles))
	}
}

func TestScanProfiles_Empty(t *testing.T) {
	dir, clean := makeTempState(t)
	defer clean()

	app := &state.AppState{RootDir: filepath.Join(dir, "empty")}
	app.InitializeBaseDirs(app.RootDir)

	if err := app.ScanProfiles(); err != nil {
		t.Fatalf("ScanProfiles: %v", err)
	}
	if len(app.Profiles) != 0 {
		t.Fatalf("expected 0 profiles, got %d", len(app.Profiles))
	}
}
