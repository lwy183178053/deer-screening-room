package provisioning

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPPeerApplierSendsScopedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/peers" || r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer provisioner-token" {
			t.Fatalf("request=%s %s auth=%q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	applier := HTTPPeerApplier{URL: server.URL, Token: "provisioner-token", Client: server.Client()}
	if err := applier.Apply(context.Background(), Peer{Name: "ugreen-media", Address: "10.77.0.3", PublicKey: "public-key"}); err != nil {
		t.Fatal(err)
	}
}
