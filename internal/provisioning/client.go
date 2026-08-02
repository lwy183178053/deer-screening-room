package provisioning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Peer struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	PublicKey string `json:"public_key"`
}

type PeerApplier interface {
	Apply(context.Context, Peer) error
	Revoke(context.Context, string, ...string) error
}

type HTTPPeerApplier struct {
	URL    string
	Token  string
	Client *http.Client
}

func (a HTTPPeerApplier) Apply(ctx context.Context, peer Peer) error {
	return a.do(ctx, http.MethodPost, "/v1/peers", peer)
}

func (a HTTPPeerApplier) Revoke(ctx context.Context, name string, address ...string) error {
	path := "/v1/peers/" + name
	if len(address) > 0 && strings.TrimSpace(address[0]) != "" {
		path += "?address=" + url.QueryEscape(address[0])
	}
	return a.do(ctx, http.MethodDelete, path, nil)
}

func (a HTTPPeerApplier) do(ctx context.Context, method, path string, body any) error {
	var payload *bytes.Reader
	if body == nil {
		payload = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(a.URL, "/")+path, payload)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+a.Token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	client := a.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode/100 != 2 {
		return fmt.Errorf("wireguard provisioner returned %s", response.Status)
	}
	return nil
}
