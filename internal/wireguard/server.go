package wireguard

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type Server struct {
	ConfigPath string
	Interface  string
	Token      string
	Sync       func([]byte) error
	mu         sync.Mutex
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/peers", s.apply)
	mux.HandleFunc("/v1/peers/", s.remove)
	return mux
}

func (s *Server) apply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.authorized(r) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var peer Peer
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&peer); err != nil {
		writeServerError(w, http.StatusUnprocessableEntity, "invalid_peer")
		return
	}
	if err := s.update(func(config []byte) ([]byte, error) { return ApplyPeer(config, peer) }); err != nil {
		writeServerError(w, http.StatusInternalServerError, "peer_apply_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) remove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.authorized(r) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	name, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/v1/peers/"))
	if err != nil || name == "" {
		writeServerError(w, http.StatusUnprocessableEntity, "invalid_peer")
		return
	}
	address := strings.TrimSpace(r.URL.Query().Get("address"))
	remove := func(config []byte) ([]byte, error) { return RemovePeer(config, name) }
	if address != "" {
		remove = func(config []byte) ([]byte, error) { return RemovePeerByAddress(config, name, address) }
	}
	if err := s.update(remove); err != nil {
		writeServerError(w, http.StatusInternalServerError, "peer_remove_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) update(transform func([]byte) ([]byte, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(s.ConfigPath) == "" {
		return errors.New("wireguard config path is required")
	}
	original, err := os.ReadFile(s.ConfigPath)
	if err != nil {
		return err
	}
	updated, err := transform(original)
	if err != nil {
		return err
	}
	backup := s.ConfigPath + ".bak"
	if err := os.WriteFile(backup, original, 0600); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(s.ConfigPath), ".wg0.conf-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(updated); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, s.ConfigPath); err != nil {
		return err
	}
	syncConfig := s.Sync
	if syncConfig == nil {
		syncConfig = s.syncConfig
	}
	if err := syncConfig(updated); err != nil {
		_ = os.WriteFile(s.ConfigPath, original, 0600)
		_ = syncConfig(original)
		return err
	}
	return nil
}

func (s *Server) syncConfig(_ []byte) error {
	interfaceName := s.Interface
	if interfaceName == "" {
		interfaceName = "wg0"
	}
	strip, err := exec.Command("wg-quick", "strip", s.ConfigPath).Output()
	if err != nil {
		return fmt.Errorf("wg-quick strip: %w", err)
	}
	command := exec.Command("wg", "syncconf", interfaceName, "/dev/stdin")
	command.Stdin = bytes.NewReader(strip)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("wg syncconf: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (s *Server) authorized(r *http.Request) bool {
	received := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return received != "" && len(received) == len(s.Token) && subtle.ConstantTimeCompare([]byte(received), []byte(s.Token)) == 1
}

func writeServerError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code}})
}
