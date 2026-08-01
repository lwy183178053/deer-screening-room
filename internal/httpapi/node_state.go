package httpapi

import (
	"sync"
	"time"

	"deerroom/internal/store"
)

type nodeRuntimeState struct {
	ScanStatus        string
	LastScanAt        time.Time
	ScanError         string
	InventoryRevision string
}

type nodeStateStore struct {
	mu     sync.RWMutex
	states map[string]nodeRuntimeState
}

func newNodeStateStore() *nodeStateStore {
	return &nodeStateStore{states: make(map[string]nodeRuntimeState)}
}

func (s *nodeStateStore) update(name, status string, lastScanAt time.Time, scanError, revision string) {
	if status == "" {
		status = "unknown"
	}
	s.mu.Lock()
	s.states[name] = nodeRuntimeState{ScanStatus: status, LastScanAt: lastScanAt, ScanError: scanError, InventoryRevision: revision}
	s.mu.Unlock()
}

func (s *nodeStateStore) response(items []store.Node) []nodeAdminResponse {
	result := make([]nodeAdminResponse, len(items))
	s.mu.RLock()
	defer s.mu.RUnlock()
	for index, item := range items {
		state := s.states[item.Name]
		status := state.ScanStatus
		if status == "" {
			status = "unknown"
		}
		result[index] = nodeAdminResponse{Node: item, ScanStatus: status, ScanError: state.ScanError, InventoryRevision: state.InventoryRevision}
		if !state.LastScanAt.IsZero() {
			lastScanAt := state.LastScanAt
			result[index].LastScanAt = &lastScanAt
		}
	}
	return result
}

type nodeAdminResponse struct {
	store.Node
	ScanStatus        string     `json:"scan_status"`
	LastScanAt        *time.Time `json:"last_scan_at,omitempty"`
	ScanError         string     `json:"scan_error,omitempty"`
	InventoryRevision string     `json:"inventory_revision,omitempty"`
}
