package inmemory

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.OutboxStore = (*OutboxRepository)(nil)

type OutboxRepository struct {
	mu          sync.RWMutex
	coordinator *txCoordinator
	data        map[string]port.OutboxMessage
	order       []string
	cfg         outboxConfig
}

type outboxRepositoryState struct {
	Data  map[string]port.OutboxMessage
	Order []string
}

type outboxConfig struct {
	processingTimeout time.Duration
}

type OutboxRepositoryOption func(*outboxConfig)

func WithProcessingTimeout(timeout time.Duration) OutboxRepositoryOption {
	return func(cfg *outboxConfig) {
		if timeout > 0 {
			cfg.processingTimeout = timeout
		}
	}
}

func NewOutboxRepository(opts ...OutboxRepositoryOption) *OutboxRepository {
	cfg := outboxConfig{
		processingTimeout: 30 * time.Second,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(&cfg)
	}
	return &OutboxRepository{
		coordinator: newTxCoordinator(),
		data:        make(map[string]port.OutboxMessage),
		order:       make([]string, 0),
		cfg:         cfg,
	}
}

func (r *OutboxRepository) attachCoordinator(coordinator *txCoordinator) {
	r.coordinator = coordinator
}

func (r *OutboxRepository) Append(messages ...port.OutboxMessage) error {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.append(messages...)
}

func (r *OutboxRepository) append(messages ...port.OutboxMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for _, message := range messages {
		if message.ID == "" {
			continue
		}
		if _, exists := r.data[message.ID]; exists {
			continue
		}
		copied := cloneOutboxMessage(message)
		if copied.OccurredAt.IsZero() {
			copied.OccurredAt = now
		}
		if copied.NextAttemptAt.IsZero() {
			copied.NextAttemptAt = copied.OccurredAt
		}
		if copied.Status == "" {
			copied.Status = port.OutboxStatusPending
		}
		r.data[copied.ID] = copied
		r.order = append(r.order, copied.ID)
	}
	return nil
}

func (r *OutboxRepository) FetchReady(limit int, now time.Time) ([]port.OutboxMessage, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()

	r.mu.Lock()
	defer r.mu.Unlock()

	if limit <= 0 {
		return []port.OutboxMessage{}, nil
	}
	ready := make([]port.OutboxMessage, 0, limit)
	for _, id := range r.order {
		message, exists := r.data[id]
		if !exists {
			continue
		}
		if message.Status == port.OutboxStatusDead {
			continue
		}
		if message.Status == port.OutboxStatusProcessing {
			if message.NextAttemptAt.After(now) {
				continue
			}
			// stale processing reclaim
			message.Status = port.OutboxStatusPending
			r.data[id] = message
		}
		if message.Status != port.OutboxStatusPending {
			continue
		}
		if message.NextAttemptAt.After(now) {
			continue
		}
		message.Status = port.OutboxStatusProcessing
		message.AttemptCount++
		message.NextAttemptAt = now.Add(r.cfg.processingTimeout)
		r.data[id] = message
		ready = append(ready, cloneOutboxMessage(message))
		if len(ready) >= limit {
			break
		}
	}
	return ready, nil
}

func (r *OutboxRepository) SelectByID(id string) (*port.OutboxMessage, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()

	r.mu.RLock()
	defer r.mu.RUnlock()
	message, exists := r.data[id]
	if !exists {
		return nil, nil
	}
	cloned := cloneOutboxMessage(message)
	return &cloned, nil
}

func (r *OutboxRepository) SelectDead(limit int, lastID string) ([]port.OutboxMessage, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()

	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 {
		return []port.OutboxMessage{}, nil
	}

	dead := make([]port.OutboxMessage, 0, len(r.data))
	for _, message := range r.data {
		if message.Status != port.OutboxStatusDead {
			continue
		}
		dead = append(dead, cloneOutboxMessage(message))
	}
	sort.Slice(dead, func(i, j int) bool {
		if dead[i].OccurredAt.Equal(dead[j].OccurredAt) {
			return dead[i].ID > dead[j].ID
		}
		return dead[i].OccurredAt.After(dead[j].OccurredAt)
	})

	start := 0
	if strings.TrimSpace(lastID) != "" {
		start = len(dead)
		for idx, message := range dead {
			if message.ID == lastID {
				start = idx + 1
				break
			}
		}
	}
	if start >= len(dead) {
		return []port.OutboxMessage{}, nil
	}
	end := start + limit
	if end > len(dead) {
		end = len(dead)
	}
	return dead[start:end], nil
}

func (r *OutboxRepository) RenewProcessing(id string, nextAttemptAt time.Time) error {
	r.coordinator.enter()
	defer r.coordinator.exit()

	if id == "" {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	message, exists := r.data[id]
	if !exists || message.Status != port.OutboxStatusProcessing {
		return nil
	}
	message.NextAttemptAt = nextAttemptAt
	r.data[id] = message
	return nil
}

func (r *OutboxRepository) MarkSucceeded(ids ...string) error {
	r.coordinator.enter()
	defer r.coordinator.exit()

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(ids) == 0 {
		return nil
	}
	deleted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		delete(r.data, id)
		deleted[id] = struct{}{}
	}
	if len(deleted) == 0 {
		return nil
	}
	filtered := r.order[:0]
	for _, id := range r.order {
		if _, removed := deleted[id]; removed {
			continue
		}
		filtered = append(filtered, id)
	}
	r.order = filtered
	return nil
}

func (r *OutboxRepository) MarkRetry(id string, nextAttemptAt time.Time, err string) error {
	r.coordinator.enter()
	defer r.coordinator.exit()

	r.mu.Lock()
	defer r.mu.Unlock()

	message, exists := r.data[id]
	if !exists {
		return nil
	}
	message.Status = port.OutboxStatusPending
	message.NextAttemptAt = nextAttemptAt
	message.LastError = strings.TrimSpace(err)
	r.data[id] = message
	if !containsOutboxID(r.order, id) {
		r.order = append(r.order, id)
	}
	return nil
}

func (r *OutboxRepository) MarkDead(id string, err string) error {
	r.coordinator.enter()
	defer r.coordinator.exit()

	r.mu.Lock()
	defer r.mu.Unlock()

	message, exists := r.data[id]
	if !exists {
		return nil
	}
	message.Status = port.OutboxStatusDead
	message.LastError = strings.TrimSpace(err)
	r.data[id] = message
	// dead 메시지는 보존하되 polling hot-path 스캔에서 제외한다.
	filtered := r.order[:0]
	for _, itemID := range r.order {
		if itemID == id {
			continue
		}
		filtered = append(filtered, itemID)
	}
	r.order = filtered
	return nil
}

func (r *OutboxRepository) snapshot() outboxRepositoryState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state := outboxRepositoryState{
		Data:  make(map[string]port.OutboxMessage, len(r.data)),
		Order: make([]string, len(r.order)),
	}
	for id, message := range r.data {
		state.Data[id] = cloneOutboxMessage(message)
	}
	copy(state.Order, r.order)
	return state
}

func (r *OutboxRepository) restore(state outboxRepositoryState) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.data = make(map[string]port.OutboxMessage, len(state.Data))
	for id, message := range state.Data {
		r.data[id] = cloneOutboxMessage(message)
	}
	r.order = make([]string, len(state.Order))
	copy(r.order, state.Order)
}

func cloneOutboxMessage(message port.OutboxMessage) port.OutboxMessage {
	copied := message
	if message.Payload != nil {
		copied.Payload = append([]byte(nil), message.Payload...)
	}
	return copied
}

func containsOutboxID(ids []string, target string) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}
