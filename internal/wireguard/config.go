package wireguard

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type Peer struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	PublicKey string `json:"public_key"`
}

var peerNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,62}[a-z0-9]$`)
var addressPattern = regexp.MustCompile(`^10\.77\.0\.(?:[2-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-4])$`)

func ApplyPeer(config []byte, peer Peer) ([]byte, error) {
	if err := validatePeer(peer); err != nil {
		return nil, err
	}
	base, peers := splitPeers(string(config))
	filtered := make([]string, 0, len(peers)+1)
	marker := "# deer-node:" + peer.Name
	for _, section := range peers {
		if strings.Contains(section, marker) {
			continue
		}
		if strings.TrimSpace(section) != "" {
			filtered = append(filtered, strings.TrimSpace(section))
		}
	}
	filtered = append(filtered, fmt.Sprintf("%s\nPublicKey = %s\nAllowedIPs = %s/32", marker, peer.PublicKey, peer.Address))
	return joinPeers(base, filtered), nil
}

func RemovePeer(config []byte, name string) ([]byte, error) {
	if !peerNamePattern.MatchString(name) {
		return nil, errors.New("invalid peer name")
	}
	base, peers := splitPeers(string(config))
	marker := "# deer-node:" + name
	filtered := make([]string, 0, len(peers))
	for _, section := range peers {
		if strings.Contains(section, marker) {
			continue
		}
		if strings.TrimSpace(section) != "" {
			filtered = append(filtered, strings.TrimSpace(section))
		}
	}
	return joinPeers(base, filtered), nil
}

func RemovePeerByAddress(config []byte, name, address string) ([]byte, error) {
	if !peerNamePattern.MatchString(name) || !addressPattern.MatchString(address) {
		return nil, errors.New("invalid peer")
	}
	base, peers := splitPeers(string(config))
	marker := "# deer-node:" + name
	allowed := "AllowedIPs = " + address + "/32"
	filtered := make([]string, 0, len(peers))
	for _, section := range peers {
		if strings.Contains(section, marker) || strings.Contains(section, allowed) {
			continue
		}
		if strings.TrimSpace(section) != "" {
			filtered = append(filtered, strings.TrimSpace(section))
		}
	}
	return joinPeers(base, filtered), nil
}

func validatePeer(peer Peer) error {
	if !peerNamePattern.MatchString(peer.Name) || !addressPattern.MatchString(peer.Address) || strings.TrimSpace(peer.PublicKey) == "" {
		return errors.New("invalid wireguard peer")
	}
	return nil
}

func splitPeers(config string) (string, []string) {
	config = strings.ReplaceAll(config, "\r\n", "\n")
	parts := strings.Split(config, "\n[Peer]\n")
	base := strings.TrimRight(parts[0], "\n")
	if len(parts) == 1 {
		return base, nil
	}
	return base, parts[1:]
}

func joinPeers(base string, peers []string) []byte {
	result := strings.TrimRight(base, "\n")
	for _, peer := range peers {
		result += "\n\n[Peer]\n" + strings.TrimRight(peer, "\n")
	}
	return []byte(result + "\n")
}
