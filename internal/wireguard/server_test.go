package wireguard

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestServerAppliesAuthenticatedPeer(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "wg0.conf")
	if err := os.WriteFile(path, []byte("[Interface]\nAddress = 10.77.0.1/24\n"), 0600); err != nil {
		t.Fatal(err)
	}
	server := &Server{ConfigPath: path, Token: "token", Sync: func([]byte) error { return nil }}
	request := httptest.NewRequest(http.MethodPost, "/v1/peers", bytes.NewBufferString(`{"name":"ugreen-media","address":"10.77.0.3","public_key":"public"}`))
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	content, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(content, []byte("deer-node:ugreen-media")) {
		t.Fatalf("config=%q err=%v", content, err)
	}
}

func TestServerRejectsWrongToken(t *testing.T) {
	server := &Server{ConfigPath: filepath.Join(t.TempDir(), "wg0.conf"), Token: "token"}
	request := httptest.NewRequest(http.MethodPost, "/v1/peers", bytes.NewBufferString(`{"name":"ugreen-media","address":"10.77.0.3","public_key":"public"}`))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", response.Code)
	}
}
