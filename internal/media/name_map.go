package media

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const mediaMapFilename = ".deer-media-map.json"

var hashedMediaPattern = regexp.MustCompile(`^media-[0-9a-f]{64}\.[a-z0-9]+$`)

type mediaNameEntry struct {
	Studio       string `json:"studio"`
	OriginalName string `json:"original_name"`
	Title        string `json:"title"`
}

type mediaNameMap struct {
	Version int                       `json:"version"`
	Files   map[string]mediaNameEntry `json:"files"`
}

func emptyMediaNameMap() mediaNameMap {
	return mediaNameMap{Version: 1, Files: map[string]mediaNameEntry{}}
}

func loadMediaNameMap(root string) (mediaNameMap, error) {
	body, err := os.ReadFile(filepath.Join(root, mediaMapFilename))
	if errors.Is(err, os.ErrNotExist) {
		return emptyMediaNameMap(), nil
	}
	if err != nil {
		return mediaNameMap{}, err
	}
	var names mediaNameMap
	if err := json.Unmarshal(body, &names); err != nil {
		return mediaNameMap{}, fmt.Errorf("decode %s: %w", mediaMapFilename, err)
	}
	if names.Version == 0 {
		names.Version = 1
	}
	if names.Version != 1 {
		return mediaNameMap{}, fmt.Errorf("unsupported %s version %d", mediaMapFilename, names.Version)
	}
	if names.Files == nil {
		names.Files = map[string]mediaNameEntry{}
	}
	return names, nil
}

func saveMediaNameMap(root string, names mediaNameMap) error {
	if names.Version == 0 {
		names.Version = 1
	}
	if names.Files == nil {
		names.Files = map[string]mediaNameEntry{}
	}
	body, err := json.MarshalIndent(names, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(root, mediaMapFilename+".tmp-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempName)
	}()
	if err := temp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temp.Write(body); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, filepath.Join(root, mediaMapFilename))
}

func normalizedMediaPath(relative string) string {
	return strings.ToLower(filepath.ToSlash(filepath.Clean(relative)))
}

func hashedMediaFilename(relative string) string {
	digest := sha256.Sum256([]byte(normalizedMediaPath(relative)))
	ext := strings.ToLower(filepath.Ext(relative))
	return "media-" + hex.EncodeToString(digest[:]) + ext
}

func isHashedMediaFilename(name string) bool {
	return hashedMediaPattern.MatchString(strings.ToLower(name))
}

func normalizeMediaFile(root, path, relative string, names *mediaNameMap) (string, error) {
	if isHashedMediaFilename(filepath.Base(path)) {
		return path, nil
	}
	target := filepath.Join(filepath.Dir(path), hashedMediaFilename(relative))
	if target == path {
		return path, nil
	}
	if _, err := os.Stat(target); err == nil {
		return "", fmt.Errorf("media target already exists: %s", target)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.Rename(path, target); err != nil {
		return "", err
	}
	if names.Files == nil {
		names.Files = map[string]mediaNameEntry{}
	}
	key := filepath.Base(target)
	previous, hadPrevious := names.Files[key]
	parts := strings.Split(filepath.ToSlash(relative), "/")
	studio := "未分类"
	if len(parts) > 1 {
		studio = parts[0]
	}
	names.Files[key] = mediaNameEntry{
		Studio:       studio,
		OriginalName: filepath.Base(path),
		Title:        strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
	}
	if err := saveMediaNameMap(root, *names); err != nil {
		if hadPrevious {
			names.Files[key] = previous
		} else {
			delete(names.Files, key)
		}
		_ = os.Rename(target, path)
		return "", err
	}
	return target, nil
}

func mappedMediaEntry(names mediaNameMap, filename string) (mediaNameEntry, bool) {
	entry, ok := names.Files[filepath.Base(filename)]
	return entry, ok
}
