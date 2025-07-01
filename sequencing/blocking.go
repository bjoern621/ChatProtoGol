package sequencing

import (
	"net/netip"
	"sync"
)

var blockerManager = struct {
	mu      sync.Mutex
	blocked map[SequenceBlocker]bool
}{
	blocked: make(map[SequenceBlocker]bool),
}

// SequenceBlocker is a struct that provides state to block the sending of packets of a specific message type until the previous sent packets are acknowledged.
type SequenceBlocker struct {
	destinationAddr netip.Addr
	msgType         byte
}

func GetSequenceBlocker(destAddr netip.Addr, msgType byte) *SequenceBlocker {
	return &SequenceBlocker{
		destinationAddr: destAddr,
		msgType:         msgType,
	}
}

// ClearBlockers clears all blockers for the given destination address.
func ClearBlockers(destAddr netip.Addr) {
	blockerManager.mu.Lock()
	defer blockerManager.mu.Unlock()

	for b := range blockerManager.blocked {
		if b.destinationAddr == destAddr {
			delete(blockerManager.blocked, b)
		}
	}
}

// Block blocks tries to set the blocker to the blocked state.
// If the blocker is already blocked, it returns false, indicating that another message of the same type is currently being sent.
// If the blocker is not blocked, it sets the blocker to the blocked state and returns true
func (b *SequenceBlocker) Block() bool {
	blockerManager.mu.Lock()
	defer blockerManager.mu.Unlock()

	if _, exists := blockerManager.blocked[*b]; exists {
		// Already blocked, meaning another message of the same type is currently being sent.
		return false
	}

	blockerManager.blocked[*b] = true

	return true
}

// Unblock removes the blocker from the blocked state.
// If the blocker isn't blocked, this is a no-op.
func (b *SequenceBlocker) Unblock() {
	blockerManager.mu.Lock()
	defer blockerManager.mu.Unlock()

	delete(blockerManager.blocked, *b)
}
