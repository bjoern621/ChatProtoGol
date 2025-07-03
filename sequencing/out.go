package sequencing

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"sort"
	"sync"
	"time"

	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/util/assert"
	"bjoernblessin.de/chatprotogol/util/logger"
	"bjoernblessin.de/chatprotogol/util/observer"
)

// OpenAck represents an open acknowledgment for a specific addr and packet number.
type OpenAck struct {
	timer      *time.Timer
	retries    int
	observable *observer.Observable[bool]
}

type OutgoingPktNumHandler struct {
	packetNumbers                map[netip.Addr]uint32 // Maps a host address to the last packet number that was used for that host.
	openAcks                     map[netip.Addr]map[uint32]*OpenAck
	mu                           sync.Mutex
	highestAckedContiguousPktNum map[netip.Addr]int64 // Maps a host address to the highest packet number that has been acknowledged for that host.
	senderWindow                 int64
}

func NewOutgoingPktNumHandler() *OutgoingPktNumHandler {
	return &OutgoingPktNumHandler{
		packetNumbers:                make(map[netip.Addr]uint32),
		openAcks:                     make(map[netip.Addr]map[uint32]*OpenAck),
		highestAckedContiguousPktNum: make(map[netip.Addr]int64),
		senderWindow:                 common.SENDER_WINDOW,
	}
}

// ClearPacketNumbers clears the current packet number and open acknowledgments for the given peer.
// ACK observers are notified that the connection is closed (ACK not received).
// Can be called concurrently.
func (h *OutgoingPktNumHandler) ClearPacketNumbers(addr netip.Addr) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.packetNumbers, addr)

	if acks, exists := h.openAcks[addr]; exists {
		for seqNum, ack := range acks {
			// ack.timer == nil is an undesired state that shouldn't be possible at best, but it may happen if we called SubscribeToReceivedAck() but never AddOpenAck()
			if ack.timer != nil {
				ack.timer.Stop()
			}
			if ack.observable != nil {
				ack.observable.NotifyObservers(false) // Notify observers that the connection is closed
			}
			delete(h.openAcks[addr], seqNum)
		}
	}
}

// GetNextpacketNumber returns the next packet number for the given address.
// Can be called concurrently.
func (h *OutgoingPktNumHandler) GetNextpacketNumber(addr netip.Addr) [4]byte {
	h.mu.Lock()
	defer h.mu.Unlock()

	seqNum, exists := h.packetNumbers[addr]
	if !exists {
		seqNum = 0
	}

	h.packetNumbers[addr] = seqNum + 1

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
// Should only be called once per packet.
func (h *OutgoingPktNumHandler) AddOpenAck(packet *pkt.Packet, resendFunc func()) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	addr := netip.AddrFrom4(packet.Header.DestAddr)
	pktNum := packet.Header.PktNum

	pktNum64 := int64(binary.BigEndian.Uint32(pktNum[:]))

	highestAcked, ok := h.highestAckedContiguousPktNum[addr]
	if !ok {
		highestAcked = -1 // No packets have been acknowledged yet for this address
		h.highestAckedContiguousPktNum[addr] = highestAcked
	}

	if pktNum64-highestAcked > h.senderWindow {
		return errors.New("Packet number exceeds sender window")
	}

	openAck := h.createOpenAckIfNotExists(addr, pktNum)

	assert.Assert(openAck.timer == nil, "Open acknowledgment for host %s with packet number %v already exists", addr, pktNum)

	openAck.timer = time.AfterFunc(time.Second*common.ACK_TIMEOUT_SECONDS, func() { h.handleAckTimeout(addr, pktNum, resendFunc) })

	return nil
}

// createOpenAckIfNotExists creates a new OpenAck for the given address and packet number if it does not already exist.
// It initializes the retries. Timer and observable are set to nil initially.
func (h *OutgoingPktNumHandler) createOpenAckIfNotExists(addr netip.Addr, pktNum [4]byte) *OpenAck {
	if _, exists := h.openAcks[addr]; !exists {
		h.openAcks[addr] = map[uint32]*OpenAck{}
	}

	pktNum32 := binary.BigEndian.Uint32(pktNum[:])

	if _, exists := h.openAcks[addr][pktNum32]; !exists {
		h.openAcks[addr][pktNum32] = &OpenAck{
			timer:      nil,
			retries:    common.RETRIES_PER_PACKET,
			observable: nil,
		}
	}

	return h.openAcks[addr][pktNum32]
}

// handleAckTimeout is called when an acknowledgment timeout occurs.
func (h *OutgoingPktNumHandler) handleAckTimeout(addr netip.Addr, pktNum [4]byte, resendFunc func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	pktNum32 := binary.BigEndian.Uint32(pktNum[:])

	openAck, exists := h.openAcks[addr][pktNum32]
	if !exists {
		return // The open acknowledgment has been removed already, no need to handle the timeout // TODO this seems to happen but if it happens, is returning the right thing?
	}

	logger.Debugf("ACK timeout for host %s with packet number %v\n", addr, pktNum)

	resendFunc()

	openAck.retries--
	if openAck.retries <= 0 {
		logger.Warnf("Removing open acknowledgment for host %s with packet number %v after retries exhausted\n", addr, pktNum)
		h.removeOpenAck(addr, pktNum, false)
		return
	}

	openAck.timer.Reset(time.Second * common.ACK_TIMEOUT_SECONDS)
}

// RemoveOpenAck removes a packet from the open acknowledgments and notifies all observers that an ACK was received.
// If the packet number does not exist, it does nothing.
// Advances the highest acknowledged contiguous packet number if possible.
// Can be called concurrently.
func (h *OutgoingPktNumHandler) RemoveOpenAck(addr netip.Addr, pktNum [4]byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.removeOpenAck(addr, pktNum, true)
}

func (h *OutgoingPktNumHandler) removeOpenAck(addr netip.Addr, pktNum [4]byte, ackReceived bool) {
	pktNum32 := binary.BigEndian.Uint32(pktNum[:])

	openAck, exists := h.openAcks[addr][pktNum32]
	if !exists {
		logger.Warnf("Tried to remove open acknowledgment for host %s with packet number %v, but it does not exist", addr, pktNum)
		return
	}

	// ack.timer == nil is an undesired state that shouldn't be possible at best, but it may happen if we called SubscribeToReceivedAck() but never AddOpenAck()
	if openAck.timer != nil {
		openAck.timer.Stop()
	}
	if openAck.observable != nil {
		openAck.observable.NotifyObservers(ackReceived) // Notify observers that the ACK was received / not received
	}

	delete(h.openAcks[addr], pktNum32)
	if len(h.openAcks[addr]) == 0 {
		delete(h.openAcks, addr)
	}

	// Advance highest if acked packets are now contiguous
	for {
		openAcks, hasOpenAcks := h.openAcks[addr]

		nextHighestPktNum32 := uint32(h.highestAckedContiguousPktNum[addr] + 1)

		_, hasNextOpenAck := openAcks[nextHighestPktNum32]

		if !hasOpenAcks || hasNextOpenAck {
			break
		}

		delete(openAcks, nextHighestPktNum32)
		if len(openAcks) == 0 {
			delete(h.openAcks, addr)
		}

		h.highestAckedContiguousPktNum[addr]++
	}
}

// SubscribeToReceivedAck subscribes to the observable for a specific packet.
// The channel will once receive a notification when the ACK for the given packet is received or when all timeouts for the ACK have expired.
// The channel will receive a boolean value indicating whether the ACK was received (true) or (false) if no ACK was received after all timeouts expired.
func (h *OutgoingPktNumHandler) SubscribeToReceivedAck(packet *pkt.Packet) chan bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	addr := netip.AddrFrom4(packet.Header.DestAddr)
	pktNum := packet.Header.PktNum

	openAck := h.createOpenAckIfNotExists(addr, pktNum)

	if openAck.observable == nil {
		openAck.observable = observer.NewObservable[bool](1)
	}

	return openAck.observable.SubscribeOnce()
}

// GetOpenAcks returns a map of peers to their open acknowledgment packet numbers.
// This is thread-safe.
func (h *OutgoingPktNumHandler) GetOpenAcks() map[netip.Addr][]uint32 {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := make(map[netip.Addr][]uint32)
	for addr, acks := range h.openAcks {
		if len(acks) > 0 {
			pktNums := make([]uint32, 0, len(acks))
			for pktNum := range acks {
				pktNums = append(pktNums, pktNum)
			}
			// Sort for consistent output
			sort.Slice(pktNums, func(i, j int) bool { return pktNums[i] < pktNums[j] })
			result[addr] = pktNums
		}
	}
	return result
}
