package config

import "testing"

func TestParseNodeCredentials(t *testing.T) {
	credentials, err := parseNodeCredentials(`{"node-a":{"api_token":"api-a","relay_token":"relay-a","base_url":"http://10.77.0.2:8081/"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(credentials) != 1 || credentials["node-a"].BaseURL != "http://10.77.0.2:8081" {
		t.Fatalf("credentials=%+v", credentials)
	}
	if _, err := parseNodeCredentials(`{"node-a":{"api_token":"","relay_token":"relay-a","base_url":"http://node"}}`); err == nil {
		t.Fatal("accepted incomplete credentials")
	}
}
