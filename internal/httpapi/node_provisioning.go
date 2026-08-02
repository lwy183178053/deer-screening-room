package httpapi

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"deerroom/internal/provisioning"
	"deerroom/internal/store"
)

var nodeNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,62}[a-z0-9]$`)

func (a *API) provisionNode(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.adminMutation(w, r)
	if !ok {
		return
	}
	if len(a.nodeSecretsKey) != 32 || a.peerApplier == nil || strings.TrimSpace(a.cloudPublicKey) == "" || strings.TrimSpace(a.cloudEndpoint) == "" {
		writeError(w, http.StatusServiceUnavailable, "node_provisioning_unavailable", "节点自动配置尚未完成")
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &input); err != nil || !validNodeName(input.Name) {
		writeError(w, http.StatusUnprocessableEntity, "invalid_node_name", "节点名称需使用小写字母、数字、点、下划线或短横线")
		return
	}
	if _, err := a.store.ProvisionedNodeByName(r.Context(), input.Name); err == nil {
		writeError(w, http.StatusConflict, "node_name_taken", "节点名称已存在")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		storeError(w, err)
		return
	}
	usedURLs, err := a.store.ListNodeBaseURLs(r.Context())
	if err != nil {
		storeError(w, err)
		return
	}
	address, err := nextAddressFromURLs(usedURLs)
	if err != nil {
		writeError(w, http.StatusConflict, "wireguard_address_exhausted", "WireGuard 地址池已用尽")
		return
	}
	nodePrivateKey, nodePublicKey, err := provisioning.GenerateWireGuardKeyPair()
	if err != nil {
		storeError(w, err)
		return
	}
	apiToken, err := randomToken(32)
	if err != nil {
		storeError(w, err)
		return
	}
	relayToken, err := randomToken(32)
	if err != nil {
		storeError(w, err)
		return
	}
	apiSealed, err := provisioning.Seal(a.nodeSecretsKey, apiToken)
	if err != nil {
		storeError(w, err)
		return
	}
	privateKeySealed, err := provisioning.Seal(a.nodeSecretsKey, nodePrivateKey)
	if err != nil {
		storeError(w, err)
		return
	}
	relaySealed, err := provisioning.Seal(a.nodeSecretsKey, relayToken)
	if err != nil {
		storeError(w, err)
		return
	}
	peer := provisioning.Peer{Name: input.Name, Address: address, PublicKey: nodePublicKey}
	if err := a.peerApplier.Apply(r.Context(), peer); err != nil {
		writeError(w, http.StatusBadGateway, "wireguard_apply_failed", "WireGuard Peer 应用失败")
		return
	}
	baseURL := "http://" + address + ":8081"
	node, err := a.store.CreateProvisionedNode(r.Context(), input.Name, baseURL, address, nodePublicKey, privateKeySealed, apiSealed, relaySealed, a.now())
	if err != nil {
		_ = a.peerApplier.Revoke(context.Background(), input.Name)
		storeError(w, err)
		return
	}
	_ = a.store.AddAudit(r.Context(), actor.ID, "node.provisioned", "media_node", input.Name, map[string]any{"address": address})
	writeJSON(w, http.StatusCreated, map[string]any{
		"node":          node,
		"bundle_url":    "/api/v1/admin/nodes/" + itoa64(node.ID) + "/bundle",
		"download_once": true,
	})
}

func (a *API) downloadNodeBundle(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.adminRead(w, r); !ok {
		return
	}
	if len(a.nodeSecretsKey) != 32 {
		writeError(w, http.StatusServiceUnavailable, "node_provisioning_unavailable", "节点安装包服务尚未完成配置")
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	node, err := a.store.ProvisionedNodeByID(r.Context(), id)
	if err != nil {
		storeError(w, err)
		return
	}
	if node.Revoked {
		writeError(w, http.StatusConflict, "node_revoked", "节点已撤销")
		return
	}
	apiToken, err := provisioning.Open(a.nodeSecretsKey, node.APITokenSealed)
	if err != nil {
		storeError(w, err)
		return
	}
	relayToken, err := provisioning.Open(a.nodeSecretsKey, node.RelayTokenSealed)
	if err != nil {
		storeError(w, err)
		return
	}
	nodePrivateKey, err := provisioning.Open(a.nodeSecretsKey, node.WireGuardPrivateKeySealed)
	if err != nil {
		storeError(w, err)
		return
	}
	body, err := provisioning.BuildBundle(provisioning.BundleData{
		NodeName: node.Name, NodeAddress: node.WireGuardAddress, GatewayAddress: defaultString(a.gatewayAddress, "10.77.0.1"),
		GatewayURL: defaultString(a.gatewayURL, "http://10.77.0.1:8080"), CloudEndpoint: a.cloudEndpoint,
		CloudPublicKey: a.cloudPublicKey, NodePrivateKey: nodePrivateKey, NodeAPIToken: apiToken,
		RelayToken: relayToken, Image: a.bundleImage, Version: a.bundleVersion,
	})
	if err != nil {
		storeError(w, err)
		return
	}
	if err := a.store.MarkNodeBundleDownloaded(r.Context(), id, a.now()); err != nil {
		storeError(w, err)
		return
	}
	filename := strings.ReplaceAll(node.Name, " ", "-") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (a *API) rotateNode(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.adminMutation(w, r)
	if !ok {
		return
	}
	if len(a.nodeSecretsKey) != 32 {
		writeError(w, http.StatusServiceUnavailable, "node_provisioning_unavailable", "节点安装包服务尚未完成配置")
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := a.store.ProvisionedNodeByID(r.Context(), id); err != nil {
		storeError(w, err)
		return
	}
	apiToken, err := randomToken(32)
	if err != nil {
		storeError(w, err)
		return
	}
	relayToken, err := randomToken(32)
	if err != nil {
		storeError(w, err)
		return
	}
	apiSealed, err := provisioning.Seal(a.nodeSecretsKey, apiToken)
	if err != nil {
		storeError(w, err)
		return
	}
	relaySealed, err := provisioning.Seal(a.nodeSecretsKey, relayToken)
	if err != nil {
		storeError(w, err)
		return
	}
	if err := a.store.RotateProvisionedNode(r.Context(), id, apiSealed, relaySealed); err != nil {
		storeError(w, err)
		return
	}
	_ = a.store.AddAudit(r.Context(), actor.ID, "node.credentials_rotated", "media_node", itoa64(id), map[string]any{})
	writeJSON(w, http.StatusOK, map[string]any{"bundle_url": "/api/v1/admin/nodes/" + itoa64(id) + "/bundle", "download_once": true})
}

func (a *API) revokeNode(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.adminMutation(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	node, err := a.store.NodeByID(r.Context(), id, a.now())
	if err != nil {
		storeError(w, err)
		return
	}
	address := node.WireGuardAddress
	if address == "" {
		if parsed, parseErr := url.Parse(node.BaseURL); parseErr == nil {
			address = parsed.Hostname()
		}
	}
	address = wireGuardHostAddress(address)
	if a.peerApplier != nil {
		if err := a.peerApplier.Revoke(r.Context(), node.Name, address); err != nil {
			writeError(w, http.StatusBadGateway, "wireguard_apply_failed", "WireGuard Peer 撤销失败")
			return
		}
	}
	if err := a.store.DeleteNode(r.Context(), id); err != nil {
		storeError(w, err)
		return
	}
	_ = a.store.AddAudit(r.Context(), actor.ID, "node.revoked", "media_node", node.Name, map[string]any{})
	w.WriteHeader(http.StatusNoContent)
}

func validNodeName(name string) bool {
	return nodeNamePattern.MatchString(strings.TrimSpace(name))
}

func wireGuardHostAddress(address string) string {
	address = strings.TrimSpace(address)
	if host, _, err := net.ParseCIDR(address); err == nil {
		return host.String()
	}
	return address
}

func nextAddressFromURLs(urls []string) (string, error) {
	used := make([]string, 0, len(urls))
	for _, raw := range urls {
		parsed, err := url.Parse(raw)
		if err == nil && parsed.Hostname() != "" {
			used = append(used, parsed.Hostname())
		}
	}
	return provisioning.NextWireGuardAddress(used)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func itoa64(value int64) string {
	return strconv.FormatInt(value, 10)
}
