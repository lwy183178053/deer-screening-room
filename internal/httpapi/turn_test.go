package httpapi

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"deerroom/internal/media"
)

func TestTurnServersUseTemporaryRESTCredentials(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	api := &API{
		now: func() time.Time { return now }, turnURLs: []string{"turn:turn.example.test:3478?transport=udp"},
		turnSecret: "turn-secret", turnTTL: 2 * time.Hour,
	}
	servers := api.turnServers()
	if len(servers) != 1 || len(servers[0].URLs) != 1 {
		t.Fatalf("servers=%+v", servers)
	}
	wantUsername := "1785672000:deerroom"
	if servers[0].Username != wantUsername {
		t.Fatalf("username=%q want=%q", servers[0].Username, wantUsername)
	}
	digest := hmac.New(sha1.New, []byte("turn-secret"))
	_, _ = digest.Write([]byte(wantUsername))
	wantCredential := base64.StdEncoding.EncodeToString(digest.Sum(nil))
	if servers[0].Credential != wantCredential {
		t.Fatalf("credential=%v want=%q", servers[0].Credential, wantCredential)
	}
}

func TestTurnServersAddSTUNOnlyWhenDirectP2PIsEnabled(t *testing.T) {
	api := &API{now: func() time.Time { return time.Unix(100, 0) }, turnURLs: []string{"turn:turn.example.test:3478"}, stunURLs: []string{"stun:stun.example.test:3478"}, turnSecret: "secret", turnTTL: time.Hour}
	if got := api.turnServers(); len(got) != 1 || got[0].URLs[0] != "turn:turn.example.test:3478" {
		t.Fatalf("relay servers=%+v", got)
	}
	api.p2pEnabled.Store(true)
	got := api.turnServers()
	if len(got) != 2 || got[0].URLs[0] != "stun:stun.example.test:3478" || got[1].URLs[0] != "turn:turn.example.test:3478" {
		t.Fatalf("direct servers=%+v", got)
	}
}

func TestNodeOfferPayloadUsesGatewayICEPolicy(t *testing.T) {
	api := &API{
		now: func() time.Time { return time.Unix(100, 0) }, turnURLs: []string{"turn:trusted.example.test:3478"},
		turnSecret: "turn-secret", turnTTL: time.Hour,
	}
	body, err := api.nodeOfferPayload("session", "media-key", "offer-sdp", "offer")
	if err != nil {
		t.Fatal(err)
	}
	var offer media.WebRTCOffer
	if err := json.Unmarshal(body, &offer); err != nil {
		t.Fatal(err)
	}
	if offer.AllowDirect || len(offer.ICEServers) != 1 || offer.ICEServers[0].URLs[0] != "turn:trusted.example.test:3478" {
		t.Fatalf("relay offer=%+v", offer)
	}
	api.p2pEnabled.Store(true)
	body, err = api.nodeOfferPayload("session", "media-key", "offer-sdp", "offer")
	if err != nil || json.Unmarshal(body, &offer) != nil || !offer.AllowDirect {
		t.Fatalf("direct offer=%+v err=%v", offer, err)
	}
}
