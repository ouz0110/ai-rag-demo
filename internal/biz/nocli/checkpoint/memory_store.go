package checkpoint

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MemoryStore 内存型 Checkpoint 存储实现 (线程安全)
type MemoryStore struct {
	mu          sync.RWMutex
	checkpoints map[string]*SubAgentCheckpoint
}

var _ ICheckpointStore = (*MemoryStore)(nil)

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		checkpoints: make(map[string]*SubAgentCheckpoint),
	}
}

func (s *MemoryStore) Save(ctx context.Context, cp *SubAgentCheckpoint) error {
	if cp == nil || cp.SessionID == "" {
		return fmt.Errorf("invalid checkpoint data: session_id is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkpoints[cp.SessionID] = cp
	return nil
}

func (s *MemoryStore) Get(ctx context.Context, sessionID string) (*SubAgentCheckpoint, bool, error) {
	if sessionID == "" {
		return nil, false, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp, ok := s.checkpoints[sessionID]
	if !ok || cp == nil {
		return nil, false, nil
	}

	// 🎯 24 小时 TTL 过期检查: 超时自动算作失效
	if cp.CreatedAt > 0 && time.Now().Unix()-cp.CreatedAt > 86400 {
		return nil, false, nil
	}

	return cp, true, nil
}

func (s *MemoryStore) Delete(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.checkpoints, sessionID)
	return nil
}
