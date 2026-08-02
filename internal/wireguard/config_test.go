package wireguard

import (
	"strings"
	"testing"
)

func TestApplyPeerReplacesOnlyMarkedPeer(t *testing.T) {
	initial := "[Interface]\nAddress = 10.77.0.1/24\n\n[Peer]\nPublicKey = old\nAllowedIPs = 10.77.0.2/32\n"
	updated, err := ApplyPeer([]byte(initial), Peer{Name: "ugreen-media", Address: "10.77.0.3", PublicKey: "new"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "# deer-node:ugreen-media") || !strings.Contains(string(updated), "AllowedIPs = 10.77.0.3/32") || !strings.Contains(string(updated), "PublicKey = old") {
		t.Fatalf("updated=%q", updated)
	}
	updated, err = ApplyPeer(updated, Peer{Name: "ugreen-media", Address: "10.77.0.4", PublicKey: "newer"})
	if err != nil || strings.Count(string(updated), "# deer-node:ugreen-media") != 1 || strings.Contains(string(updated), "10.77.0.3/32") {
		t.Fatalf("replacement=%q err=%v", updated, err)
	}
}

func TestRemovePeerKeepsOtherPeers(t *testing.T) {
	initial := "[Interface]\nAddress = 10.77.0.1/24\n\n[Peer]\n# deer-node:one\nPublicKey = one\nAllowedIPs = 10.77.0.2/32\n\n[Peer]\n# deer-node:two\nPublicKey = two\nAllowedIPs = 10.77.0.3/32\n"
	updated, err := RemovePeer([]byte(initial), "one")
	if err != nil || strings.Contains(string(updated), "deer-node:one") || !strings.Contains(string(updated), "deer-node:two") {
		t.Fatalf("updated=%q err=%v", updated, err)
	}
}
