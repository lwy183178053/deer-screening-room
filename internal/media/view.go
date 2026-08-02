package media

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const mediaNameLimitBytes = 240

type MediaView struct {
	sourceRoot string
	viewRoot   string
}

func NewMediaView(sourceRoot, viewRoot string) (*MediaView, error) {
	sourceRoot, err := filepath.Abs(filepath.Clean(sourceRoot))
	if err != nil {
		return nil, err
	}
	viewRoot, err = filepath.Abs(filepath.Clean(viewRoot))
	if err != nil {
		return nil, err
	}
	if sourceRoot == viewRoot {
		return nil, errors.New("media source and view roots must differ")
	}
	if err := os.MkdirAll(viewRoot, 0o750); err != nil {
		return nil, err
	}
	return &MediaView{sourceRoot: sourceRoot, viewRoot: viewRoot}, nil
}

func (v *MediaView) Sync() error {
	if info, err := os.Stat(v.sourceRoot); err != nil || !info.IsDir() {
		if err != nil {
			return fmt.Errorf("media source root: %w", err)
		}
		return errors.New("media source root is not a directory")
	}

	desired := map[string]struct{}{}
	entries := make([]titleManifestEntry, 0)
	err := filepath.WalkDir(v.sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != v.sourceRoot && isWithin(path, v.viewRoot) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !videoExtensions[ext] {
			return nil
		}
		relative, err := filepath.Rel(v.sourceRoot, path)
		if err != nil {
			return err
		}
		viewRelative := mediaViewPath(relative)
		if _, exists := desired[viewRelative]; exists {
			return fmt.Errorf("media view alias collision: %s", viewRelative)
		}
		destination := filepath.Join(v.viewRoot, filepath.FromSlash(viewRelative))
		if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
			return err
		}
		if err := ensureMediaLink(path, destination); err != nil {
			return fmt.Errorf("link %s: %w", relative, err)
		}
		desired[viewRelative] = struct{}{}
		entries = append(entries, titleManifestEntry{Path: viewRelative, Title: strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))})
		return nil
	})
	if err != nil {
		return err
	}
	if err := removeStaleMediaLinks(v.viewRoot, desired); err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	body, err := json.Marshal(titleManifest{Entries: entries})
	if err != nil {
		return err
	}
	temp := filepath.Join(v.viewRoot, ".catalog-titles.json.tmp")
	if err := os.WriteFile(temp, body, 0o600); err != nil {
		return err
	}
	manifest := filepath.Join(v.viewRoot, "catalog-titles.json")
	if err := os.Rename(temp, manifest); err != nil {
		if removeErr := os.Remove(manifest); removeErr != nil && !os.IsNotExist(removeErr) {
			_ = os.Remove(temp)
			return err
		}
		if retryErr := os.Rename(temp, manifest); retryErr != nil {
			_ = os.Remove(temp)
			return retryErr
		}
	}
	return nil
}

func mediaViewPath(relative string) string {
	base := filepath.Base(relative)
	if len([]byte(base)) <= mediaNameLimitBytes {
		return filepath.ToSlash(relative)
	}
	digest := sha256.Sum256([]byte(filepath.ToSlash(relative)))
	alias := "media-" + hex.EncodeToString(digest[:]) + strings.ToLower(filepath.Ext(base))
	directory := filepath.ToSlash(filepath.Dir(relative))
	if directory == "." {
		return alias
	}
	return filepath.ToSlash(filepath.Join(directory, alias))
}

func ensureMediaLink(source, destination string) error {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return err
	}
	destinationInfo, destinationErr := os.Stat(destination)
	if destinationErr == nil && os.SameFile(sourceInfo, destinationInfo) {
		return nil
	}
	if destinationErr != nil && !os.IsNotExist(destinationErr) {
		_ = os.Remove(destination)
	} else if destinationErr == nil {
		if err := os.Remove(destination); err != nil {
			return err
		}
	}
	if err := os.Link(source, destination); err == nil {
		return nil
	}
	if err := os.Symlink(source, destination); err != nil {
		return err
	}
	return nil
}

func removeStaleMediaLinks(root string, desired map[string]struct{}) error {
	var stale []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if path == filepath.Join(root, "catalog-titles.json") || path == filepath.Join(root, ".catalog-titles.json.tmp") {
			return nil
		}
		if !videoExtensions[strings.ToLower(filepath.Ext(entry.Name()))] {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if _, exists := desired[filepath.ToSlash(relative)]; !exists {
			stale = append(stale, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, path := range stale {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func isWithin(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}
