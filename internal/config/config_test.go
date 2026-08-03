package config

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestParseNodeSecretsKey(t *testing.T) {
	key := bytes.Repeat([]byte{9}, 32)
	parsed, err := parseNodeSecretsKey(base64.RawStdEncoding.EncodeToString(key))
	if err != nil || !bytes.Equal(parsed, key) {
		t.Fatalf("parsed=%x err=%v", parsed, err)
	}
	if _, err := parseNodeSecretsKey("short"); err == nil {
		t.Fatal("accepted short key")
	}
}

func TestMediaNodeDefaultsToPinnedBundleVersion(t *testing.T) {
	t.Setenv("DEER_ROLE", "media-node")
	t.Setenv("DEER_NODE_API_TOKEN", "node-token")
	t.Setenv("DEER_RELAY_TOKEN", "relay-token")
	t.Setenv("DEER_NODE_VERSION", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NodeVersion != "v0.1.4" {
		t.Fatalf("node version=%q", cfg.NodeVersion)
	}
}
