package sequencing

import (
	"net/netip"
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
	packetNumbers map[netip.Addr]uint32 // Maps a host address to the last packet number that was used for that host.
	openAcks      map[netip.Addr]map[[4]byte]*OpenAck
	mu            sync.Mutex
}

func NewOutgoingPktNumHandler() *OutgoingPktNumHandler {
	return &OutgoingPktNumHandler{
		packetNumbers: make(map[netip.Addr]uint32),
		openAcks:      make(map[netip.Addr]map[[4]byte]*OpenAck),
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
func (h *OutgoingPktNumHandler) AddOpenAck(packet *pkt.Packet, resendFunc func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	addr := netip.AddrFrom4(packet.Header.DestAddr)
	pktNum := packet.Header.PktNum

	openAck := h.createOpenAckIfNotExists(addr, pktNum)

	assert.Assert(openAck.timer == nil, "Open acknowledgment for host %s with packet number %v already exists", addr, pktNum)

	openAck.timer = time.AfterFunc(time.Second*common.ACK_TIMEOUT_SECONDS, func() { h.handleAckTimeout(addr, pktNum, resendFunc) })
}

// createOpenAckIfNotExists creates a new OpenAck for the given address and packet number if it does not already exist.
// It initializes the retries. Timer and observable are set to nil initially.
func (h *OutgoingPktNumHandler) createOpenAckIfNotExists(addr netip.Addr, pktNum [4]byte) *OpenAck {
	if _, exists := h.openAcks[addr]; !exists {
		h.openAcks[addr] = map[[4]byte]*OpenAck{}
	}

	if _, exists := h.openAcks[addr][pktNum]; !exists {
		h.openAcks[addr][pktNum] = &OpenAck{
			timer:      nil,
			retries:    common.RETRIES_PER_PACKET,
			observable: nil,
		}
	}

	return h.openAcks[addr][pktNum]
}

// handleAckTimeout is called when an acknowledgment timeout occurs.
func (h *OutgoingPktNumHandler) handleAckTimeout(addr netip.Addr, pktNum [4]byte, resendFunc func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	openAck, exists := h.openAcks[addr][pktNum]
	if !exists {
		return // The open acknowledgment has been removed already, no need to handle the timeout // TODO this seems to happen but if it happens, is returning the right thing?
	}

	logger.Warnf("ACK timeout for host %s with packet number %v\n", addr, pktNum)

	resendFunc()

	// openAck.retries--
	if openAck.retries <= 0 {
		logger.Warnf("Removing open acknowledgment for host %s with packet number %v after retries exhausted\n", addr, pktNum)
		if openAck.observable != nil {
			openAck.observable.NotifyObservers(false) // Notify observers that the ACK was not received
		}
		delete(h.openAcks[addr], pktNum) // No more retries left, remove the open acknowledgment
		return
	}

	openAck.timer.Reset(time.Second * common.ACK_TIMEOUT_SECONDS)
}

// RemoveOpenAck removes a packet from the open acknowledgments and notifies all observers that an ACK was received.
// If the packet number does not exist, it does nothing.
// Can be called concurrently.
func (h *OutgoingPktNumHandler) RemoveOpenAck(addr netip.Addr, pktNum [4]byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	openAck, exists := h.openAcks[addr][pktNum]
	if !exists {
		return
	}

	// ack.timer == nil is an undesired state that shouldn't be possible at best, but it may happen if we called SubscribeToReceivedAck() but never AddOpenAck()
	if openAck.timer != nil {
		openAck.timer.Stop()
	}
	if openAck.observable != nil {
		openAck.observable.NotifyObservers(true) // Notify observers that the ACK was received
	}
	delete(h.openAcks[addr], pktNum)
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
