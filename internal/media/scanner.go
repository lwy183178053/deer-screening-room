package media

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

type Item struct {
	MediaKey      string `json:"media_key"`
	Studio        string `json:"studio"`
	Title         string `json:"title"`
	PosterKey     string `json:"poster_key"`
	DurationMS    int64  `json:"duration_ms"`
	SizeBytes     int64  `json:"size_bytes"`
	BitRate       int64  `json:"bit_rate"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	VideoCodec    string `json:"video_codec"`
	AudioCodec    string `json:"audio_codec"`
	Compatibility string `json:"compatibility"`
}

type cachedItem struct {
	Item
	RelativePath string `json:"relative_path"`
	ModifiedUnix int64  `json:"modified_unix"`
}

type titleManifest struct {
	Entries []titleManifestEntry `json:"entries"`
}

type titleManifestEntry struct {
	Path  string `json:"path"`
	Title string `json:"title"`
}

type probeResult struct {
	DurationMS             int64
	BitRate                int64
	Width, Height          int
	VideoCodec, AudioCodec string
}

type Scanner struct {
	mediaRoot, posterRoot string
	probe                 func(string) (probeResult, error)
	poster                func(string, string) error
	mu                    sync.RWMutex
	scanMu                sync.Mutex
	paths                 map[string]string
	cache                 map[string]cachedItem
	titles                map[string]string
}

var videoExtensions = map[string]bool{".mp4": true, ".m4v": true, ".mov": true, ".mkv": true, ".webm": true, ".avi": true}

func NewScanner(mediaRoot, posterRoot string) (*Scanner, error) {
	if mediaRoot == "" || posterRoot == "" {
		return nil, errors.New("media and poster roots are required")
	}
	if err := os.MkdirAll(posterRoot, 0o750); err != nil {
		return nil, err
	}
	s := &Scanner{mediaRoot: mediaRoot, posterRoot: posterRoot, paths: map[string]string{}, cache: map[string]cachedItem{}, titles: map[string]string{}}
	s.probe = s.ffprobe
	s.poster = s.ffmpegPoster
	s.loadCache()
	s.loadTitleManifest()
	return s, nil
}

func (s *Scanner) Scan() ([]Item, error) {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	s.loadTitleManifest()
	nextPaths := map[string]string{}
	nextCache := map[string]cachedItem{}
	err := filepath.WalkDir(s.mediaRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !videoExtensions[ext] {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(s.mediaRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		key := mediaKey(rel)
		nextPaths[key] = path
		if cached, ok := s.cache[key]; ok && cached.ModifiedUnix == info.ModTime().Unix() && cached.SizeBytes == info.Size() {
			if manifestTitle := s.titles[rel]; manifestTitle != "" {
				cached.Title = manifestTitle
			}
			cached.Compatibility = mediaCompatibility(ext, cached.VideoCodec, cached.AudioCodec)
			nextCache[key] = cached
			return nil
		}
		probed, err := s.probe(path)
		if err != nil {
			probed = probeResult{}
		}
		parts := strings.Split(rel, "/")
		studio := "未分类"
		if len(parts) > 1 {
			studio = parts[0]
		}
		title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if manifestTitle := s.titles[rel]; manifestTitle != "" {
			title = manifestTitle
		}
		compatibility := mediaCompatibility(ext, probed.VideoCodec, probed.AudioCodec)
		posterKey := ""
		posterPath := filepath.Join(s.posterRoot, key+".jpg")
		if _, err := os.Stat(posterPath); err == nil {
			posterKey = key
		} else if probed.DurationMS > 0 && s.poster(path, posterPath) == nil {
			posterKey = key
		}
		nextCache[key] = cachedItem{Item: Item{MediaKey: key, Studio: studio, Title: title, PosterKey: posterKey, DurationMS: probed.DurationMS, SizeBytes: info.Size(), BitRate: probed.BitRate, Width: probed.Width, Height: probed.Height, VideoCodec: probed.VideoCodec, AudioCodec: probed.AudioCodec, Compatibility: compatibility}, RelativePath: rel, ModifiedUnix: info.ModTime().Unix()}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.paths = nextPaths
	s.cache = nextCache
	s.mu.Unlock()
	if err := s.saveCache(); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(nextCache))
	for _, cached := range nextCache {
		items = append(items, cached.Item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].MediaKey < items[j].MediaKey })
	return items, nil
}

func mediaCompatibility(ext, videoCodec, audioCodec string) string {
	if (ext == ".mp4" || ext == ".m4v" || ext == ".mov") && (videoCodec == "h264" || videoCodec == "av1") && (audioCodec == "" || audioCodec == "aac") {
		return "ready"
	}
	return "unsupported"
}

func (s *Scanner) ScanWithRevision() ([]Item, string, error) {
	items, err := s.Scan()
	if err != nil {
		return nil, "", err
	}
	type revisionItem struct {
		MediaKey     string `json:"media_key"`
		SizeBytes    int64  `json:"size_bytes"`
		ModifiedUnix int64  `json:"modified_unix"`
	}
	s.mu.RLock()
	revisionItems := make([]revisionItem, 0, len(s.cache))
	for mediaKey, cached := range s.cache {
		revisionItems = append(revisionItems, revisionItem{MediaKey: mediaKey, SizeBytes: cached.SizeBytes, ModifiedUnix: cached.ModifiedUnix})
	}
	s.mu.RUnlock()
	sort.Slice(revisionItems, func(i, j int) bool { return revisionItems[i].MediaKey < revisionItems[j].MediaKey })
	body, err := json.Marshal(revisionItems)
	if err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256(body)
	return items, base64.RawURLEncoding.EncodeToString(digest[:]), nil
}

func (s *Scanner) ResolveMedia(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path, ok := s.paths[key]
	return path, ok
}
func (s *Scanner) ResolvePoster(key string) (string, bool) {
	if !validKey(key) {
		return "", false
	}
	path := filepath.Join(s.posterRoot, key+".jpg")
	info, err := os.Stat(path)
	return path, err == nil && info.Mode().IsRegular()
}

func (s *Scanner) loadCache() {
	body, err := os.ReadFile(filepath.Join(s.posterRoot, "catalog-cache.json"))
	if err != nil {
		return
	}
	var items []cachedItem
	if json.Unmarshal(body, &items) != nil {
		return
	}
	for _, item := range items {
		s.cache[item.MediaKey] = item
	}
}

func (s *Scanner) loadTitleManifest() {
	titles := map[string]string{}
	body, err := os.ReadFile(filepath.Join(s.mediaRoot, "catalog-titles.json"))
	if err != nil {
		s.mu.Lock()
		s.titles = titles
		s.mu.Unlock()
		return
	}
	body = bytes.TrimPrefix(body, []byte{0xef, 0xbb, 0xbf})
	var manifest titleManifest
	if json.Unmarshal(body, &manifest) != nil {
		s.mu.Lock()
		s.titles = titles
		s.mu.Unlock()
		return
	}
	for _, entry := range manifest.Entries {
		path := filepath.ToSlash(strings.TrimSpace(entry.Path))
		title := strings.TrimSpace(entry.Title)
		if path != "" && title != "" {
			titles[path] = title
		}
	}
	s.mu.Lock()
	s.titles = titles
	s.mu.Unlock()
}
func (s *Scanner) saveCache() error {
	s.mu.RLock()
	items := make([]cachedItem, 0, len(s.cache))
	for _, item := range s.cache {
		items = append(items, item)
	}
	s.mu.RUnlock()
	sort.Slice(items, func(i, j int) bool { return items[i].MediaKey < items[j].MediaKey })
	body, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	temp := filepath.Join(s.posterRoot, "catalog-cache.json.tmp")
	if err := os.WriteFile(temp, body, 0o600); err != nil {
		return err
	}
	return os.Rename(temp, filepath.Join(s.posterRoot, "catalog-cache.json"))
}

func (s *Scanner) ffprobe(path string) (probeResult, error) {
	command := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration,bit_rate:stream=codec_type,codec_name,width,height", "-of", "json", path)
	output, err := command.Output()
	if err != nil {
		return probeResult{}, err
	}
	var body struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
			BitRate  string `json:"bit_rate"`
		} `json:"format"`
	}
	if err := json.Unmarshal(output, &body); err != nil {
		return probeResult{}, err
	}
	var result probeResult
	var duration float64
	_, _ = fmt.Sscanf(body.Format.Duration, "%f", &duration)
	result.DurationMS = int64(duration * 1000)
	_, _ = fmt.Sscan(body.Format.BitRate, &result.BitRate)
	for _, stream := range body.Streams {
		if stream.CodecType == "video" && result.VideoCodec == "" {
			result.VideoCodec = stream.CodecName
			result.Width = stream.Width
			result.Height = stream.Height
		}
		if stream.CodecType == "audio" && result.AudioCodec == "" {
			result.AudioCodec = stream.CodecName
		}
	}
	if result.VideoCodec == "" {
		return probeResult{}, errors.New("video stream missing")
	}
	return result, nil
}

func (s *Scanner) ffmpegPoster(source, destination string) error {
	args := []string{"-v", "error", "-y", "-ss", "5", "-i", source, "-frames:v", "1", "-vf", "scale=640:-2", "-q:v", "4", destination}
	if runtime.GOOS != "windows" {
		args = append([]string{"-n", "10", "ffmpeg"}, args...)
		return exec.Command("nice", args...).Run()
	}
	return exec.Command("ffmpeg", args...).Run()
}

func mediaKey(relative string) string {
	digest := sha256.Sum256([]byte(strings.ToLower(filepath.ToSlash(relative))))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}
func validKey(value string) bool {
	if len(value) != 43 {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(value)
	return err == nil
}
