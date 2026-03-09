package inmemory

import "sync"

type txCoordinator struct {
	mu sync.RWMutex
}

func newTxCoordinator() *txCoordinator {
	return &txCoordinator{}
}

func (c *txCoordinator) enter() {
	if c == nil {
		return
	}
	c.mu.RLock()
}

func (c *txCoordinator) exit() {
	if c == nil {
		return
	}
	c.mu.RUnlock()
}

func (c *txCoordinator) lock() {
	if c == nil {
		return
	}
	c.mu.Lock()
}

func (c *txCoordinator) unlock() {
	if c == nil {
		return
	}
	c.mu.Unlock()
}
