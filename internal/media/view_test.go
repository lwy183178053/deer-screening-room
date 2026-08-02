package media

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMediaViewSyncsLongNamesAndManifest(t *testing.T) {
	source := t.TempDir()
	view := t.TempDir()
	studio := filepath.Join(source, "studio")
	if err := os.MkdirAll(studio, 0o755); err != nil {
		t.Fatal(err)
	}
	longName := strings.Repeat("剧", 100) + ".mp4"
	longPath := filepath.Join(studio, longName)
	shortPath := filepath.Join(studio, "short.mp4")
	for _, path := range []string{longPath, shortPath} {
		if err := os.WriteFile(path, []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	mediaView, err := NewMediaView(source, view)
	if err != nil {
		t.Fatal(err)
	}
	if err := mediaView.Sync(); err != nil {
		t.Fatal(err)
	}

	manifestBody, err := os.ReadFile(filepath.Join(view, "catalog-titles.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest titleManifest
	if err := json.Unmarshal(manifestBody, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Entries) != 2 {
		t.Fatalf("manifest entries=%d", len(manifest.Entries))
	}
	longAlias := ""
	for _, entry := range manifest.Entries {
		if entry.Title == strings.TrimSuffix(longName, filepath.Ext(longName)) {
			longAlias = entry.Path
			break
		}
	}
	if longAlias == "" || len([]byte(filepath.Base(longAlias))) > 240 || filepath.Base(longAlias) == longName {
		t.Fatalf("long alias=%q", longAlias)
	}
	aliasInfo, err := os.Stat(filepath.Join(view, longAlias))
	if err != nil {
		t.Fatal(err)
	}
	sourceInfo, err := os.Stat(longPath)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(aliasInfo, sourceInfo) {
		t.Fatal("view entry is not linked to the source file")
	}

	newPath := filepath.Join(studio, "new.mp4")
	if err := os.WriteFile(newPath, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := mediaView.Sync(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(view, "studio", "new.mp4")); err != nil {
		t.Fatalf("new view entry: %v", err)
	}
	if err := os.Remove(shortPath); err != nil {
		t.Fatal(err)
	}
	if err := mediaView.Sync(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(view, "studio", "short.mp4")); !os.IsNotExist(err) {
		t.Fatalf("stale view entry still exists: %v", err)
	}
}
