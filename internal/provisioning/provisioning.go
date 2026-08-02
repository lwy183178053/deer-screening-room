package provisioning

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/curve25519"
)

const (
	gatewayAddress = "10.77.0.1"
	firstNode      = 2
	lastNode       = 254
)

type BundleData struct {
	NodeName       string
	NodeAddress    string
	GatewayAddress string
	GatewayURL     string
	CloudEndpoint  string
	CloudPublicKey string
	NodePrivateKey string
	NodeAPIToken   string
	RelayToken     string
	Image          string
	Version        string
}

func NextWireGuardAddress(used []string) (string, error) {
	occupied := make(map[string]struct{}, len(used))
	for _, value := range used {
		value = strings.TrimSpace(strings.TrimSuffix(value, "/32"))
		if value != "" {
			occupied[value] = struct{}{}
		}
	}
	for octet := firstNode; octet <= lastNode; octet++ {
		candidate := fmt.Sprintf("10.77.0.%d", octet)
		if _, exists := occupied[candidate]; !exists {
			return candidate, nil
		}
	}
	return "", errors.New("wireguard address pool is exhausted")
}

func Seal(key []byte, value string) (string, error) {
	block, err := newCipher(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, block.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := block.Seal(nonce, nonce, []byte(value), nil)
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func Open(key []byte, value string) (string, error) {
	block, err := newCipher(key)
	if err != nil {
		return "", err
	}
	sealed, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	if len(sealed) < block.NonceSize() {
		return "", errors.New("sealed value is too short")
	}
	nonce, payload := sealed[:block.NonceSize()], sealed[block.NonceSize():]
	opened, err := block.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", err
	}
	return string(opened), nil
}

func GenerateWireGuardKeyPair() (string, string, error) {
	private := make([]byte, curve25519.ScalarSize)
	if _, err := io.ReadFull(rand.Reader, private); err != nil {
		return "", "", err
	}
	public, err := curve25519.X25519(private, curve25519.Basepoint)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(private), base64.StdEncoding.EncodeToString(public), nil
}

func BuildBundle(data BundleData) ([]byte, error) {
	data.NodeAddress = strings.TrimSuffix(strings.TrimSpace(data.NodeAddress), "/32")
	data.GatewayAddress = strings.TrimSuffix(strings.TrimSpace(data.GatewayAddress), "/32")
	if strings.TrimSpace(data.NodeName) == "" || strings.TrimSpace(data.NodeAddress) == "" || strings.TrimSpace(data.NodePrivateKey) == "" || strings.TrimSpace(data.NodeAPIToken) == "" || strings.TrimSpace(data.RelayToken) == "" {
		return nil, errors.New("node bundle data is incomplete")
	}
	if data.GatewayAddress == "" {
		data.GatewayAddress = gatewayAddress
	}
	if data.GatewayURL == "" {
		data.GatewayURL = "http://" + data.GatewayAddress + ":8080"
	}
	if data.Image == "" {
		data.Image = "ghcr.io/lwy183178053/deer-screening-room"
	}
	if data.Version == "" {
		data.Version = "latest"
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	files := map[string]string{
		"node.env":     fmt.Sprintf("DEER_COMPOSE_PROJECT_NAME=deer\nDEER_IMAGE=%s\nDEER_VERSION=%s\nDEER_PULL_POLICY=missing\nDEER_CONFIG_IMAGE=alpine:3.22\nDEER_WIREGUARD_IMAGE=lscr.io/linuxserver/wireguard@sha256:ac43e1226878d2611315172d6ea357a95cb326ee73124b91108118efc8666889\nTZ=Asia/Shanghai\n\nDEER_NODE_NAME=%s\nDEER_NODE_WIREGUARD_ADDRESS=%s/32\nDEER_GATEWAY_WIREGUARD_ADDRESS=%s\nDEER_GATEWAY_URL=%s\nDEER_NODE_PUBLIC_URL=http://%s:8081\nDEER_CLOUD_ENDPOINT=%s\nDEER_CLOUD_WIREGUARD_PUBLIC_KEY=%s\nDEER_WIREGUARD_PRIVATE_KEY=%s\nDEER_NODE_API_TOKEN=%s\nDEER_RELAY_TOKEN=%s\nDEER_MEDIA_SOURCE_ROOT=/source\nDEER_MEDIA_ROOT=/media\nDEER_MEDIA_HOST_PATH=/CHANGE_ME\nDEER_POSTER_HOST_PATH=./posters\nDEER_SCAN_INTERVAL=10m\n", data.Image, data.Version, data.NodeName, data.NodeAddress, data.GatewayAddress, data.GatewayURL, data.NodeAddress, data.CloudEndpoint, data.CloudPublicKey, data.NodePrivateKey, data.NodeAPIToken, data.RelayToken),
		"compose.yaml": bundleCompose(data.Image, data.Version),
		"README.txt":   "小鹿放映室媒体节点\n\n导入此 Docker 项目后，只需要选择一次 NAS 视频目录，将它映射到容器 /source，并启动项目。节点会自动在 /media 生成兼容视图，不复制、不删除原始视频。node.env 已包含本节点的一次性连接配置，请保留在 NAS 私有目录中。\n\n节点启动后，管理员页面会在 90 秒内显示在线；首次扫描完成后视频会出现在目录。\n",
	}
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := io.WriteString(file, content); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return archive.Bytes(), nil
}

func bundleCompose(image, version string) string {
	return fmt.Sprintf(`name: deer

services:
  wg-init:
    container_name: deer-wg-init
    image: alpine:3.22
    restart: "no"
    env_file: [node.env]
    volumes: [wireguard-data:/output]
    entrypoint: ["/bin/sh", "-ec"]
    command:
      - >-
        install -d -m 700 /output/wg_confs; umask 077; printf '[Interface]\nAddress = %%s\nPrivateKey = %%s\n\n[Peer]\nPublicKey = %%s\nEndpoint = %%s\nAllowedIPs = %%s/32\nPersistentKeepalive = 25\n' "$${DEER_NODE_WIREGUARD_ADDRESS}" "$${DEER_WIREGUARD_PRIVATE_KEY}" "$${DEER_CLOUD_WIREGUARD_PUBLIC_KEY}" "$${DEER_CLOUD_ENDPOINT}" "$${DEER_GATEWAY_WIREGUARD_ADDRESS}" > /output/wg_confs/wg0.conf
    read_only: true
    tmpfs: [/tmp]

  wg:
    container_name: deer-wg
    image: lscr.io/linuxserver/wireguard@sha256:ac43e1226878d2611315172d6ea357a95cb326ee73124b91108118efc8666889
    restart: unless-stopped
    cap_add: [NET_ADMIN, SYS_MODULE]
    environment: {PUID: 0, PGID: 0, TZ: Asia/Shanghai}
    volumes: [wireguard-data:/config, /lib/modules:/lib/modules:ro]
    sysctls: {net.ipv4.conf.all.src_valid_mark: 1}
    healthcheck:
      test: ["CMD-SHELL", "wg show wg0 >/dev/null 2>&1"]
      interval: 3s
      timeout: 2s
      retries: 20
    depends_on: {wg-init: {condition: service_completed_successfully}}

  node:
    container_name: deer-node
    image: %s:%s
    restart: unless-stopped
    command: ["/app/media-node"]
    network_mode: service:wg
    env_file: [node.env]
    environment:
      DEER_ROLE: media-node
      DEER_HTTP_ADDR: ":8081"
      DEER_MEDIA_SOURCE_ROOT: /source
      DEER_MEDIA_ROOT: /media
      DEER_POSTER_ROOT: /app/posters
    volumes:
      - ${DEER_MEDIA_HOST_PATH:-/CHANGE_ME}:/source:ro
      - media-view:/media
      - ${DEER_POSTER_HOST_PATH:-./posters}:/app/posters
    depends_on: {wg: {condition: service_healthy}}
    read_only: true
    tmpfs: [/tmp]

volumes:
  wireguard-data:
  media-view:
`, image, version)
}

func newCipher(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("node secrets key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
