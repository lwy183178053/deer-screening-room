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
