package sequencing

import (
	"net/netip"
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
}

func NewOutgoingPktNumHandler() *OutgoingPktNumHandler {
	return &OutgoingPktNumHandler{
		sequenceNumbers: make(map[netip.Addr]uint32),
		openAcks:        make(map[netip.Addr]map[[4]byte]*OpenAck),
	}
}

// ClearSequenceNumbers clears the current sequence number and open acknowledgments for the given peer.
func (h *OutgoingPktNumHandler) ClearSequenceNumbers(peerAddr netip.Addr) {
	delete(h.sequenceNumbers, peerAddr)

	if acks, exists := h.openAcks[peerAddr]; exists {
		for seqNum, ack := range acks {
			ack.timer.Stop()
			delete(h.openAcks[peerAddr], seqNum)
		}
	}
}

// GetNextSequenceNumber returns the next sequence number for the given address.
func (h *OutgoingPktNumHandler) GetNextSequenceNumber(peerAddr netip.Addr) [4]byte {
	seqNum, exists := h.sequenceNumbers[peerAddr]
	if !exists {
		seqNum = 0
	}

	h.sequenceNumbers[peerAddr] = seqNum + 1

	return [4]byte{
		byte(seqNum >> 24),
		byte(seqNum >> 16),
		byte(seqNum >> 8),
		byte(seqNum),
	}
}

// AddOpenAck adds a sequence number to the open acknowledgments for the given peer and starts a new timeout timer.
// After the timeout, it will call the provided resend function to resend the packet.
func (h *OutgoingPktNumHandler) AddOpenAck(peerAddr netip.Addr, seqNum [4]byte, resendFunc func()) {
	if _, exists := h.openAcks[peerAddr]; !exists {
		h.openAcks[peerAddr] = map[[4]byte]*OpenAck{}
	}

	openAck := &OpenAck{
		timer:   time.AfterFunc(time.Second*common.ACK_TIMEOUT_SECONDS, func() { h.handleAckTimeout(peerAddr, seqNum, resendFunc) }),
		retries: common.RETRIES_PER_PACKET,
	}

	h.openAcks[peerAddr][seqNum] = openAck
}

// handleAckTimeout is called when an acknowledgment timeout occurs.
func (h *OutgoingPktNumHandler) handleAckTimeout(peerAddr netip.Addr, seqNum [4]byte, resendFunc func()) {
	logger.Warnf("ACK timeout for peer %s with sequence number %v\n", peerAddr, seqNum)

	resendFunc()

	openAck, exists := h.openAcks[peerAddr][seqNum]
	assert.Assert(exists, "No open acknowledgment found for peer %s with sequence number %v", peerAddr, seqNum) // TODO may fail?

	openAck.retries--
	if openAck.retries <= 0 {
		delete(h.openAcks[peerAddr], seqNum)
		return // No more retries left, remove the open acknowledgment
	}

	openAck.timer.Reset(time.Second * common.ACK_TIMEOUT_SECONDS)
}

// RemoveOpenAck removes a sequence number from the open acknowledgments for the given peer.
// If the sequence number does not exist, it does nothing.
func (h *OutgoingPktNumHandler) RemoveOpenAck(peerAddr netip.Addr, seqNum [4]byte) {
	openAck, exists := h.openAcks[peerAddr][seqNum]
	if !exists {
		return
	}

	openAck.timer.Stop()
	delete(h.openAcks[peerAddr], seqNum)
}
