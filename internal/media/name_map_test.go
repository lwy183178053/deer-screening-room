package media

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestHashedMediaFilenameIsStable(t *testing.T) {
	first := hashedMediaFilename("悠米/作品一.mp4")
	second := hashedMediaFilename("悠米/作品一.mp4")
	if first != second {
		t.Fatalf("hash changed: %q != %q", first, second)
	}
	if !regexp.MustCompile(`^media-[0-9a-f]{64}\.mp4$`).MatchString(first) {
		t.Fatalf("unexpected hashed filename: %q", first)
	}
	if got := hashedMediaFilename("悠米/作品一.MP4"); !strings.HasSuffix(got, ".mp4") {
		t.Fatalf("extension was not normalized: %q", got)
	}
}

func TestMediaNameMapRoundTrip(t *testing.T) {
	root := t.TempDir()
	want := mediaNameMap{Version: 1, Files: map[string]mediaNameEntry{
		"media-abcd.mp4": {Studio: "悠米", OriginalName: "作品一.mp4", OriginalPath: "悠米/作品一.mp4", Title: "作品一"},
	}}
	if err := saveMediaNameMap(root, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadMediaNameMap(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != want.Version || got.Files["media-abcd.mp4"] != want.Files["media-abcd.mp4"] {
		t.Fatalf("map=%+v want=%+v", got, want)
	}
}

func TestNormalizeMediaFilePreservesStudioAndMapsTitle(t *testing.T) {
	root := t.TempDir()
	studio := filepath.Join(root, "悠米")
	if err := os.Mkdir(studio, 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(studio, "作品一.mp4")
	if err := os.WriteFile(source, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	names := mediaNameMap{Version: 1, Files: map[string]mediaNameEntry{}}
	target, err := normalizeMediaFile(root, source, "悠米/作品一.mp4", &names)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(target) != studio || filepath.Base(target) == filepath.Base(source) {
		t.Fatalf("target=%q source=%q", target, source)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source still exists: %v", err)
	}
	entry, ok := names.Files[filepath.Base(target)]
	if !ok || entry.Studio != "悠米" || entry.OriginalName != "作品一.mp4" || entry.OriginalPath != "悠米/作品一.mp4" || entry.Title != "作品一" {
		t.Fatalf("entry=%+v ok=%v", entry, ok)
	}
	if _, err := os.Stat(filepath.Join(root, mediaMapFilename)); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeMediaFileDoesNotOverwriteConflict(t *testing.T) {
	root := t.TempDir()
	studio := filepath.Join(root, "悠米")
	if err := os.Mkdir(studio, 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(studio, "作品一.mp4")
	if err := os.WriteFile(source, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(studio, hashedMediaFilename("悠米/作品一.mp4"))
	if err := os.WriteFile(target, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	names := mediaNameMap{Version: 1, Files: map[string]mediaNameEntry{}}
	if _, err := normalizeMediaFile(root, source, "悠米/作品一.mp4", &names); err == nil {
		t.Fatal("expected destination conflict")
	}
	body, err := os.ReadFile(source)
	if err != nil || string(body) != "source" {
		t.Fatalf("source changed: %q err=%v", body, err)
	}
	body, err = os.ReadFile(target)
	if err != nil || string(body) != "existing" {
		t.Fatalf("target changed: %q err=%v", body, err)
	}
}
