package config

import "testing"

func TestParseNodeCredentials(t *testing.T) {
	credentials, err := parseNodeCredentials(`{"node-a":{"api_token":"api-a","base_url":"http://10.77.0.2:8081/"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(credentials) != 1 || credentials["node-a"].BaseURL != "http://10.77.0.2:8081" {
		t.Fatalf("credentials=%+v", credentials)
	}
	if _, err := parseNodeCredentials(`{"node-a":{"api_token":"","base_url":"http://node"}}`); err == nil {
		t.Fatal("accepted incomplete credentials")
	}
}

func TestGatewayRequiresTURNRelayConfiguration(t *testing.T) {
	t.Setenv("DEER_ROLE", "gateway")
	t.Setenv("DATABASE_URL", "postgres://database")
	t.Setenv("DEER_NODE_API_TOKEN", "node-token")
	t.Setenv("DEER_TURN_URLS", "")
	t.Setenv("DEER_TURN_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("gateway accepted missing TURN configuration")
	}
	t.Setenv("DEER_TURN_URLS", "stun:stun.example.test:3478")
	t.Setenv("DEER_TURN_SECRET", "turn-secret")
	if _, err := Load(); err == nil {
		t.Fatal("gateway accepted STUN-only configuration")
	}
	t.Setenv("DEER_TURN_URLS", "turn:turn.example.test:3478")
	if _, err := Load(); err != nil {
		t.Fatalf("gateway rejected TURN configuration: %v", err)
	}
}
