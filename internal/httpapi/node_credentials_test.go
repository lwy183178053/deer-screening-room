package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestNodeCredentialsBindNameAndAddress(t *testing.T) {
	api := &API{nodeCredentials: map[string]NodeCredential{"node-a": {APIToken: "api-a", RelayToken: "relay-a", BaseURL: "http://10.77.0.2:8081"}}}
	request := httptest.NewRequest("POST", "/api/v1/internal/media/heartbeat", nil)
	request.Header.Set("Authorization", "Bearer api-a")
	if !api.nodeAuthorized(request, "node-a", "http://10.77.0.2:8081") {
		t.Fatal("valid node rejected")
	}
	if api.nodeAuthorized(request, "node-b", "http://10.77.0.2:8081") || api.nodeAuthorized(request, "node-a", "http://10.77.0.3:8081") {
		t.Fatal("node identity or address was not bound")
	}
	if token, ok := api.relayTokenFor("node-a"); !ok || token != "relay-a" {
		t.Fatalf("relay token=%q ok=%v", token, ok)
	}
}
