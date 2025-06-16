package sequencing

import (
	"net/netip"
	"sync"
	"time"

	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/util/assert"
	"bjoernblessin.de/chatprotogol/util/logger"
)

type OpenAck struct {
	timer   *time.Timer
	retries int
}

type OutgoingPktNumHandler struct {
	sequenceNumbers map[netip.Addr]uint32
	openAcks        map[netip.Addr]map[[4]byte]*OpenAck
	mu              sync.Mutex
}

func NewOutgoingPktNumHandler() *OutgoingPktNumHandler {
	return &OutgoingPktNumHandler{
		sequenceNumbers: make(map[netip.Addr]uint32),
		openAcks:        make(map[netip.Addr]map[[4]byte]*OpenAck),
	}
}

// ClearPacketNumbers clears the current packet number and open acknowledgments for the given peer.
// Can be called concurrently.
func (h *OutgoingPktNumHandler) ClearPacketNumbers(addr netip.Addr) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.sequenceNumbers, addr)

	if acks, exists := h.openAcks[addr]; exists {
		for seqNum, ack := range acks {
			ack.timer.Stop()
			delete(h.openAcks[addr], seqNum)
		}
	}
}

// GetNextpacketNumber returns the next packet number for the given address.
// Can be called concurrently.
func (h *OutgoingPktNumHandler) GetNextpacketNumber(addr netip.Addr) [4]byte {
	h.mu.Lock()
	defer h.mu.Unlock()

	seqNum, exists := h.sequenceNumbers[addr]
	if !exists {
		seqNum = 0
	}

	h.sequenceNumbers[addr] = seqNum + 1

	return [4]byte{
		byte(seqNum >> 24),
		byte(seqNum >> 16),
		byte(seqNum >> 8),
		byte(seqNum),
	}
}

// AddOpenAck adds a sequence number to the open acknowledgments for the given peer and starts a new timeout timer.
// After the timeout, it will call the provided resend function to resend the packet.
// Can be called concurrently.
func (h *OutgoingPktNumHandler) AddOpenAck(addr netip.Addr, pktNum [4]byte, resendFunc func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.openAcks[addr]; !exists {
		h.openAcks[addr] = map[[4]byte]*OpenAck{}
	}

	openAck := &OpenAck{
		timer:   time.AfterFunc(time.Second*common.ACK_TIMEOUT_SECONDS, func() { h.handleAckTimeout(addr, pktNum, resendFunc) }),
		retries: common.RETRIES_PER_PACKET,
	}

	h.openAcks[addr][pktNum] = openAck
}

// handleAckTimeout is called when an acknowledgment timeout occurs.
func (h *OutgoingPktNumHandler) handleAckTimeout(addr netip.Addr, pktNum [4]byte, resendFunc func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	logger.Warnf("ACK timeout for host %s with sequence number %v\n", addr, pktNum)

	resendFunc()

	openAck, exists := h.openAcks[addr][pktNum]
	assert.Assert(exists, "No open acknowledgment found for host %s with packet number %v", addr, pktNum)

	openAck.retries--
	if openAck.retries <= 0 {
		delete(h.openAcks[addr], pktNum)
		return // No more retries left, remove the open acknowledgment
	}

	openAck.timer.Reset(time.Second * common.ACK_TIMEOUT_SECONDS)
}

// RemoveOpenAck removes a packet from the open acknowledgments.
// If the packet number does not exist, it does nothing.
// Can be called concurrently.
func (h *OutgoingPktNumHandler) RemoveOpenAck(addr netip.Addr, pktNum [4]byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	openAck, exists := h.openAcks[addr][pktNum]
	if !exists {
		return
	}

	openAck.timer.Stop()
	delete(h.openAcks[addr], pktNum)
}
