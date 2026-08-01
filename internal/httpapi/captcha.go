package httpapi

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	captchaAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
	captchaLifetime = 5 * time.Minute
	captchaMaxTries = 5
)

type captchaChallenge struct {
	digest    [32]byte
	clientIP  string
	purpose   string
	expiresAt time.Time
	attempts  int
}

type captchaManager struct {
	mu         sync.Mutex
	challenges map[string]captchaChallenge
	now        func() time.Time
}

func newCaptchaManager(now func() time.Time) *captchaManager {
	return &captchaManager{challenges: make(map[string]captchaChallenge), now: now}
}

func (m *captchaManager) issue(clientIP, purpose string) (string, []byte, error) {
	answer, err := randomCaptchaText(5)
	if err != nil {
		return "", nil, err
	}
	id, err := randomToken(18)
	if err != nil {
		return "", nil, err
	}
	imageBody, err := renderCaptcha(answer)
	if err != nil {
		return "", nil, err
	}
	now := m.now()
	m.mu.Lock()
	m.cleanupLocked(now)
	m.challenges[id] = captchaChallenge{digest: sha256.Sum256([]byte(answer)), clientIP: clientIP, purpose: purpose, expiresAt: now.Add(captchaLifetime)}
	m.mu.Unlock()
	return id, imageBody, nil
}

func (m *captchaManager) verify(id, answer, clientIP, purpose string) bool {
	now := m.now()
	digest := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(answer))))
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked(now)
	challenge, ok := m.challenges[id]
	if !ok || challenge.clientIP != clientIP || challenge.purpose != purpose {
		return false
	}
	challenge.attempts++
	if challenge.digest == digest {
		delete(m.challenges, id)
		return true
	}
	if challenge.attempts >= captchaMaxTries {
		delete(m.challenges, id)
	} else {
		m.challenges[id] = challenge
	}
	return false
}

func (m *captchaManager) cleanupLocked(now time.Time) {
	for id, challenge := range m.challenges {
		if !challenge.expiresAt.After(now) {
			delete(m.challenges, id)
		}
	}
}

type windowEntry struct {
	count int
	reset time.Time
}

type loginEntry struct {
	requiredUntil time.Time
}

type authGuard struct {
	mu             sync.Mutex
	register       map[string]windowEntry
	login          map[string]windowEntry
	captchaIssuing map[string]windowEntry
	loginCaptcha   map[string]loginEntry
	lastCleanup    time.Time
	now            func() time.Time
}

func newAuthGuard(now func() time.Time) *authGuard {
	return &authGuard{
		register: make(map[string]windowEntry), login: make(map[string]windowEntry),
		captchaIssuing: make(map[string]windowEntry), loginCaptcha: make(map[string]loginEntry), now: now,
	}
}

func (g *authGuard) allowRegister(ip string) (bool, time.Duration) {
	return g.allow(g.register, ip, 12, 10*time.Minute)
}

func (g *authGuard) allowLogin(ip string) (bool, time.Duration) {
	return g.allow(g.login, ip, 30, time.Minute)
}

func (g *authGuard) allowCaptcha(ip string) (bool, time.Duration) {
	return g.allow(g.captchaIssuing, ip, 60, 10*time.Minute)
}

func (g *authGuard) allow(entries map[string]windowEntry, key string, limit int, window time.Duration) (bool, time.Duration) {
	now := g.now()
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.lastCleanup.IsZero() || !g.lastCleanup.Add(time.Minute).After(now) {
		for _, entries := range []map[string]windowEntry{g.register, g.login, g.captchaIssuing} {
			for key, entry := range entries {
				if !entry.reset.After(now) {
					delete(entries, key)
				}
			}
		}
		for key, entry := range g.loginCaptcha {
			if !entry.requiredUntil.After(now) {
				delete(g.loginCaptcha, key)
			}
		}
		g.lastCleanup = now
	}
	entry := entries[key]
	if !entry.reset.After(now) {
		entry = windowEntry{reset: now.Add(window)}
	}
	if entry.count >= limit {
		return false, entry.reset.Sub(now)
	}
	entry.count++
	entries[key] = entry
	return true, 0
}

func (g *authGuard) requireLoginCaptcha(key string) bool {
	now := g.now()
	g.mu.Lock()
	defer g.mu.Unlock()
	entry, ok := g.loginCaptcha[key]
	if !ok || !entry.requiredUntil.After(now) {
		delete(g.loginCaptcha, key)
		return false
	}
	return true
}

func (g *authGuard) markLoginFailure(key string) {
	g.mu.Lock()
	g.loginCaptcha[key] = loginEntry{requiredUntil: g.now().Add(15 * time.Minute)}
	g.mu.Unlock()
}

func (g *authGuard) clearLoginFailure(key string) {
	g.mu.Lock()
	delete(g.loginCaptcha, key)
	g.mu.Unlock()
}

func (a *API) captcha(w http.ResponseWriter, r *http.Request) {
	purpose := strings.TrimSpace(r.URL.Query().Get("purpose"))
	if purpose != "login" && purpose != "register" {
		writeError(w, http.StatusUnprocessableEntity, "captcha_purpose_invalid", "验证码用途无效")
		return
	}
	ip := clientIP(r)
	if ok, retry := a.authGuard.allowCaptcha(ip); !ok {
		writeRateLimited(w, retry)
		return
	}
	id, body, err := a.captchas.issue(ip, purpose)
	if err != nil {
		storeError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]string{"captcha_id": id, "image": "data:image/png;base64," + base64.StdEncoding.EncodeToString(body)})
}

func clientIP(r *http.Request) string {
	for _, name := range []string{"CF-Connecting-IP", "X-Real-IP"} {
		if value := strings.TrimSpace(r.Header.Get(name)); net.ParseIP(value) != nil {
			return value
		}
	}
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); net.ParseIP(forwarded) != nil {
		return forwarded
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && net.ParseIP(host) != nil {
		return host
	}
	return r.RemoteAddr
}

func writeRateLimited(w http.ResponseWriter, retry time.Duration) {
	seconds := max(int(retry.Round(time.Second)/time.Second), 1)
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeError(w, http.StatusTooManyRequests, "rate_limited", "请求过于频繁，请稍后重试")
}

func randomCaptchaText(length int) (string, error) {
	result := make([]byte, length)
	for index := range result {
		value, err := rand.Int(rand.Reader, big.NewInt(int64(len(captchaAlphabet))))
		if err != nil {
			return "", err
		}
		result[index] = captchaAlphabet[value.Int64()]
	}
	return string(result), nil
}

func renderCaptcha(value string) ([]byte, error) {
	const width, height = 170, 56
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.RGBA{R: 248, G: 247, B: 244, A: 255}}, image.Point{}, draw.Src)
	parsed, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: 28, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, err
	}
	defer face.Close()
	for index, character := range value {
		y := 37 + randomSmall(9) - 4
		ink := color.RGBA{R: uint8(35 + randomSmall(65)), G: uint8(35 + randomSmall(50)), B: uint8(35 + randomSmall(55)), A: 255}
		drawer := &font.Drawer{Dst: canvas, Src: &image.Uniform{C: ink}, Face: face, Dot: fixed.P(17+index*29+randomSmall(5), y)}
		drawer.DrawString(string(character))
	}
	for index := 0; index < 5; index++ {
		drawNoiseLine(canvas, randomSmall(width), randomSmall(height), randomSmall(width), randomSmall(height))
	}
	for index := 0; index < 90; index++ {
		x, y := randomSmall(width), randomSmall(height)
		canvas.Set(x, y, color.RGBA{R: uint8(90 + randomSmall(130)), G: uint8(80 + randomSmall(130)), B: uint8(80 + randomSmall(130)), A: 170})
	}
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func randomSmall(maximum int) int {
	if maximum <= 1 {
		return 0
	}
	value, err := rand.Int(rand.Reader, big.NewInt(int64(maximum)))
	if err != nil {
		return 0
	}
	return int(value.Int64())
}

func drawNoiseLine(canvas *image.RGBA, x0, y0, x1, y1 int) {
	ink := color.RGBA{R: uint8(130 + randomSmall(100)), G: uint8(100 + randomSmall(100)), B: uint8(100 + randomSmall(100)), A: 170}
	dx, sx := abs(x1-x0), 1
	if x0 > x1 {
		sx = -1
	}
	dy, sy := -abs(y1-y0), 1
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		if image.Pt(x0, y0).In(canvas.Bounds()) {
			canvas.Set(x0, y0, ink)
		}
		if x0 == x1 && y0 == y1 {
			return
		}
		twice := 2 * err
		if twice >= dy {
			err += dy
			x0 += sx
		}
		if twice <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
