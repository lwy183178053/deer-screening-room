package provisioning

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"io"
	"strings"
	"testing"
)

func TestNextWireGuardAddressSkipsUsedAddresses(t *testing.T) {
	address, err := NextWireGuardAddress([]string{"10.77.0.1/32", "10.77.0.2/32", "10.77.0.4/32"})
	if err != nil {
		t.Fatal(err)
	}
	if address != "10.77.0.3" {
		t.Fatalf("address=%q", address)
	}
}

func TestSealOpenRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	sealed, err := Seal(key, "secret-value")
	if err != nil {
		t.Fatal(err)
	}
	opened, err := Open(key, sealed)
	if err != nil || opened != "secret-value" {
		t.Fatalf("opened=%q err=%v", opened, err)
	}
	if _, err := Open(bytes.Repeat([]byte{8}, 32), sealed); err == nil {
		t.Fatal("expected wrong key to fail")
	}
}

func TestGenerateWireGuardKeyPair(t *testing.T) {
	privateKey, publicKey, err := GenerateWireGuardKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := base64.StdEncoding.DecodeString(privateKey); err != nil {
		t.Fatalf("private key: %v", err)
	}
	if _, err := base64.StdEncoding.DecodeString(publicKey); err != nil {
		t.Fatalf("public key: %v", err)
	}
	if privateKey == publicKey || len(privateKey) == 0 || len(publicKey) == 0 {
		t.Fatalf("invalid key pair: %q %q", privateKey, publicKey)
	}
}

func TestBuildBundleContainsOnlyRuntimeFiles(t *testing.T) {
	body, err := BuildBundle(BundleData{
		NodeName: "ugreen-media", NodeAddress: "10.77.0.3", GatewayAddress: "10.77.0.1",
		GatewayURL: "http://10.77.0.1:8080", CloudEndpoint: "38.34.191.104:51820",
		CloudPublicKey: "cloud-public", NodePrivateKey: "node-private", NodeAPIToken: "api-token", RelayToken: "relay-token",
		Image: "ghcr.io/example/deer:v1", Version: "v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[file.Name] = string(content)
	}
	if !strings.Contains(files["node.env"], "DEER_NODE_NAME=ugreen-media") || !strings.Contains(files["node.env"], "DEER_MEDIA_HOST_PATH=/CHANGE_ME") || strings.Contains(files["node.env"], "DEER_MEDIA_SOURCE_ROOT") {
		t.Fatalf("env=%q", files["node.env"])
	}
	if strings.Contains(files["compose.yaml"], "replace-node-api-token") || !strings.Contains(files["compose.yaml"], "env_file: [node.env]") || strings.Contains(files["compose.yaml"], "DEER_MEDIA_SOURCE_ROOT") || strings.Contains(files["compose.yaml"], "media-view") || !strings.Contains(files["compose.yaml"], "DEER_MEDIA_HOST_PATH") || !strings.Contains(files["compose.yaml"], ":/media:rw") {
		t.Fatalf("compose did not expose runtime env mapping")
	}
	if !strings.Contains(files["README.txt"], "只需要选择一次 NAS 视频目录") || !strings.Contains(files["README.txt"], ".deer-media-map.json") {
		t.Fatalf("readme=%q", files["README.txt"])
	}
}

func TestBuildBundleNormalizesWireGuardCIDR(t *testing.T) {
	body, err := BuildBundle(BundleData{
		NodeName: "node", NodeAddress: "10.77.0.2/32", GatewayAddress: "10.77.0.1/32",
		NodePrivateKey: "private", NodeAPIToken: "api", RelayToken: "relay",
		CloudEndpoint: "38.34.191.104:51820", CloudPublicKey: "cloud",
	})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range archive.File {
		if file.Name != "node.env" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "DEER_NODE_WIREGUARD_ADDRESS=10.77.0.2/32\n") || strings.Contains(string(content), "10.77.0.2/32/32") {
			t.Fatalf("env=%q", content)
		}
		return
	}
	t.Fatal("node.env not found")
}
