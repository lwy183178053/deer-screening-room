package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestNodeCredentialsBindNameAndAddress(t *testing.T) {
	api := &API{nodeCredentials: map[string]NodeCredential{"node-a": {APIToken: "api-a", BaseURL: "http://10.77.0.2:8081"}}}
	request := httptest.NewRequest("POST", "/api/v1/internal/media/heartbeat", nil)
	request.Header.Set("Authorization", "Bearer api-a")
	if !api.nodeAuthorized(request, "node-a", "http://10.77.0.2:8081") {
		t.Fatal("valid node rejected")
	}
	if api.nodeAuthorized(request, "node-b", "http://10.77.0.2:8081") || api.nodeAuthorized(request, "node-a", "http://10.77.0.3:8081") {
		t.Fatal("node identity or address was not bound")
	}
	if token := api.nodeTokenFor("node-a"); token != "api-a" {
		t.Fatalf("node token=%q", token)
	}
}
