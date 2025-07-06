package sequencing

import (
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"math"
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
	cwnd                         map[netip.Addr]int64
	ssthresh                     map[netip.Addr]int64
	cAvoidanceAcc                map[netip.Addr]int64     // Used to count the number of packets acked in congestion avoidance phase
	lastCongestionEventTime      map[netip.Addr]time.Time // Timestamp of the last congestion event
	initialCwnd                  int64
}

func NewOutgoingPktNumHandler(initialCwnd int64) *OutgoingPktNumHandler {
	return &OutgoingPktNumHandler{
		packetNumbers:                make(map[netip.Addr]uint32),
		openAcks:                     make(map[netip.Addr]map[uint32]*OpenAck),
		highestAckedContiguousPktNum: make(map[netip.Addr]int64),
		cwnd:                         make(map[netip.Addr]int64),
		ssthresh:                     make(map[netip.Addr]int64),
		cAvoidanceAcc:                make(map[netip.Addr]int64),
		lastCongestionEventTime:      make(map[netip.Addr]time.Time),
		initialCwnd:                  initialCwnd,
	}
}

// ClearPacketNumbers clears the current packet number and open acknowledgments for the given peer.
// ACK observers are notified that the connection is closed (ACK not received).
// Can be called concurrently.
func (h *OutgoingPktNumHandler) ClearPacketNumbers(addr netip.Addr) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.packetNumbers, addr)
	delete(h.cwnd, addr)
	delete(h.ssthresh, addr)
	delete(h.cAvoidanceAcc, addr)
	delete(h.highestAckedContiguousPktNum, addr)
	delete(h.lastCongestionEventTime, addr)

	if acks, exists := h.openAcks[addr]; exists {
		for seqNum, ack := range acks {
			ack.timer.Stop()
			ack.observable.NotifyObservers(false) // Notify observers that the connection is closed

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
func (h *OutgoingPktNumHandler) AddOpenAck(packet *pkt.Packet, resendFunc func()) (chan bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	addr := netip.AddrFrom4(packet.Header.DestAddr)
	pktNum := packet.Header.PktNum
	pktNum32 := binary.BigEndian.Uint32(pktNum[:])
	pktNum64 := int64(binary.BigEndian.Uint32(pktNum[:]))

	_, exists := h.openAcks[addr][pktNum32]
	assert.Assert(!exists, "Open acknowledgment for host", addr, "with packet number", pktNum, "already exists")

	highestAcked, ok := h.highestAckedContiguousPktNum[addr]
	if !ok {
		highestAcked = -1 // No packets have been acknowledged yet for this address
		h.highestAckedContiguousPktNum[addr] = highestAcked
	}

	cwnd, ok := h.cwnd[addr]
	if !ok {
		cwnd = h.initialCwnd
		h.cwnd[addr] = cwnd
	}
	if pktNum64-highestAcked > cwnd {
		return nil, errors.New("Packet number " +
			fmt.Sprint(pktNum64) +
			" exceeds congestion window, [" +
			fmt.Sprint(highestAcked) + ", " +
			fmt.Sprint(highestAcked+cwnd) + "]")
	}

	openAck := h.createOpenAck(addr, pktNum)

	openAck.timer = time.AfterFunc(common.ACK_TIMEOUT_DURATION, func() { h.handleAckTimeout(addr, pktNum, resendFunc) })

	return openAck.observable.SubscribeOnce(), nil
}

// createOpenAck creates a new OpenAck for the given address and packet number.
// It initializes the retries and observable. Timer is set to nil initially.
func (h *OutgoingPktNumHandler) createOpenAck(addr netip.Addr, pktNum [4]byte) *OpenAck {
	pktNum32 := binary.BigEndian.Uint32(pktNum[:])

	if _, exists := h.openAcks[addr]; !exists {
		h.openAcks[addr] = make(map[uint32]*OpenAck)
	}

	h.openAcks[addr][pktNum32] = &OpenAck{
		timer:      nil,
		retries:    common.RETRIES_PER_PACKET,
		observable: observer.NewObservable[bool](1),
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

	if openAck.retries == common.RETRIES_PER_PACKET { // React only if the packet hasn't been resent yet (https://datatracker.ietf.org/doc/html/rfc5681#section-3.1)
		if time.Since(h.lastCongestionEventTime[addr]) > common.ACK_TIMEOUT_DURATION { // Simulate: per peer RTO
			// Multiplicative decrease
			cwnd := h.cwnd[addr]
			h.ssthresh[addr] = max(cwnd/2, 2)
			h.cwnd[addr] = max(cwnd/2, h.initialCwnd)
			h.cAvoidanceAcc[addr] = 0 // Reset accumulator after congestion event
			logger.Debugf("CONGESTION EVENT for %s %d: Cwnd: %d, ssthresh set to %d, cwnd reset to %d", addr, pktNum32, cwnd, h.ssthresh[addr], h.cwnd[addr])

			h.lastCongestionEventTime[addr] = time.Now()
		} else {
			logger.Debugf("Ignoring subsequent timeout for %s; within RTO cooldown period.", addr)
		}
	}

	resendFunc()

	openAck.retries--
	if openAck.retries == 0 {
		logger.Warnf("Removing open acknowledgment for host %s with packet number %v after retries exhausted\n", addr, pktNum)
		h.removeOpenAck(addr, pktNum, false)
		return
	}

	openAck.timer.Reset(common.ACK_TIMEOUT_DURATION)
}

// RemoveOpenAck removes a packet from the open acknowledgments and notifies all observers that an ACK was received.
// If the packet number does not exist, it does nothing.
// Advances the highest acknowledged contiguous packet number if possible.
// Can be called concurrently.
func (h *OutgoingPktNumHandler) RemoveOpenAck(addr netip.Addr, pktNum [4]byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	_, exists := h.openAcks[addr][binary.BigEndian.Uint32(pktNum[:])]
	if !exists {
		return
	}

	h.removeOpenAck(addr, pktNum, true)
}

// removeOpenAck removes a packet from the open acknowledgments and notifies all observers that an ACK was received or not received.
// If the packet number does not exist, it panics.
// See alternative impl at the end of this file for a second version that solves the "wrong highestAcked after congestion event" issue.
func (h *OutgoingPktNumHandler) removeOpenAck(addr netip.Addr, pktNum [4]byte, ackReceived bool) {
	pktNum32 := binary.BigEndian.Uint32(pktNum[:])

	openAck, exists := h.openAcks[addr][pktNum32]
	assert.Assert(exists, "Open acknowledgment for host %s with packet number %v does not exist", addr, pktNum)

	openAck.timer.Stop()
	openAck.observable.NotifyObservers(ackReceived) // Notify observers that the ACK was received / not received

	delete(h.openAcks[addr], pktNum32)
	if len(h.openAcks[addr]) == 0 {
		delete(h.openAcks, addr)
	}

	// Advance highest if acked packets are now contiguous
	for {
		openAcks := h.openAcks[addr]

		nextHighestPktNum32 := uint32(h.highestAckedContiguousPktNum[addr] + 1)

		_, hasNextOpenAck := openAcks[nextHighestPktNum32]

		if hasNextOpenAck {
			break
		}

		currentPktNum := h.packetNumbers[addr] - 1 // Last packet sent
		if nextHighestPktNum32 > currentPktNum {
			break // We've reached the end of sent packets
		}

		h.highestAckedContiguousPktNum[addr]++
	}

	if ackReceived {
		if _, exists := h.ssthresh[addr]; !exists {
			h.ssthresh[addr] = math.MaxInt64
		}

		cwnd := h.cwnd[addr]
		ssthresh := h.ssthresh[addr]

		if cwnd < ssthresh {
			// Slow start
			h.cwnd[addr] = h.cwnd[addr] + 1
			h.cAvoidanceAcc[addr] = 0 // Reset accumulator when leaving slow start
		} else {
			// Congestion avoidance
			accu := h.cAvoidanceAcc[addr]
			accu++

			if accu >= cwnd {
				h.cwnd[addr] = h.cwnd[addr] + 1
				// h.cAvoidanceAcc[addr] = 0 // This is faster (effectively always in slow start modus)
				accu = 0 // But this should be correct
			}

			h.cAvoidanceAcc[addr] = accu
		}
	}
}

// OpenAckInfo provides public information about an open acknowledgment.
type OpenAckInfo struct {
	PktNum      uint32
	TimerStatus string
}

// GetOpenAcks returns a map of peers to their open acknowledgment packet numbers and timer status.
// This is thread-safe.
func (h *OutgoingPktNumHandler) GetOpenAcks() map[netip.Addr][]OpenAckInfo {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := make(map[netip.Addr][]OpenAckInfo)
	for addr, acks := range h.openAcks {
		if len(acks) > 0 {
			ackInfos := make([]OpenAckInfo, 0, len(acks))
			for pktNum, ack := range acks {
				status := "nil"
				if ack.timer != nil {
					status = "active"
				}
				ackInfos = append(ackInfos, OpenAckInfo{PktNum: pktNum, TimerStatus: status})
			}
			// Sort for consistent output
			sort.Slice(ackInfos, func(i, j int) bool { return ackInfos[i].PktNum < ackInfos[j].PktNum })
			result[addr] = ackInfos
		}
	}
	return result
}

// GetCongestionWindows returns a map of peers to their current congestion window size.
// This is thread-safe.
func (h *OutgoingPktNumHandler) GetCongestionWindows() map[netip.Addr]int64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Return a copy to prevent race conditions on the original map
	windowsCopy := make(map[netip.Addr]int64, len(h.cwnd))
	maps.Copy(windowsCopy, h.cwnd)
	return windowsCopy
}

// GetSlowStartThresholds returns a map of peers to their current slow start threshold.
// This is thread-safe.
func (h *OutgoingPktNumHandler) GetSlowStartThresholds() map[netip.Addr]int64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Return a copy to prevent race conditions on the original map
	thresholdsCopy := make(map[netip.Addr]int64, len(h.ssthresh))
	maps.Copy(thresholdsCopy, h.ssthresh)
	return thresholdsCopy
}

// func (h *OutgoingPktNumHandler) removeOpenAck(addr netip.Addr, pktNum [4]byte, ackReceived bool) {
// 	pktNum32 := binary.BigEndian.Uint32(pktNum[:])

// 	openAck, exists := h.openAcks[addr][pktNum32]
// 	assert.Assert(exists, "Open acknowledgment for host %s with packet number %v does not exist", addr, pktNum)

// 	openAck.timer.Stop()
// 	openAck.observable.NotifyObservers(ackReceived) // Notify observers that the ACK was received / not received

// 	// Capture state before deletion for advancement logic
// 	remainingPackets := make(map[uint32]bool)
// 	if acks, exists := h.openAcks[addr]; exists {
// 		for pktNum := range acks {
// 			if pktNum != pktNum32 { // Exclude the packet we're about to delete
// 				remainingPackets[pktNum] = true
// 			}
// 		}
// 	}

// 	delete(h.openAcks[addr], pktNum32)
// 	if len(h.openAcks[addr]) == 0 {
// 		delete(h.openAcks, addr)
// 	}

// 	// Add debug logging before advancing
// 	oldHighest := h.highestAckedContiguousPktNum[addr]

// 	// Advance highest if acked packets are now contiguous
// 	for {
// 		nextHighestPktNum32 := uint32(h.highestAckedContiguousPktNum[addr] + 1)

// 		// Check if the next packet is still pending (in remaining packets)
// 		if remainingPackets[nextHighestPktNum32] {
// 			break // Next packet is still pending, can't advance further
// 		}

// 		// Check if we've reached the highest packet number sent
// 		currentPktNum := h.packetNumbers[addr] - 1
// 		if nextHighestPktNum32 > currentPktNum {
// 			break // We've reached the end of sent packets
// 		}

// 		h.highestAckedContiguousPktNum[addr]++
// 	}

// 	// Log the advancement
// 	newHighest := h.highestAckedContiguousPktNum[addr]
// 	if newHighest != oldHighest {
// 		logger.Tracef("Advanced highest contiguous for %s from %d to %d (ACKed: %d)",
// 			addr, oldHighest, newHighest, pktNum32)
// 	}

// 	if ackReceived {
// 		if _, exists := h.ssthresh[addr]; !exists {
// 			h.ssthresh[addr] = math.MaxInt64
// 		}

// 		cwnd := h.cwnd[addr]
// 		ssthresh := h.ssthresh[addr]

// 		if cwnd < ssthresh {
// 			// Slow start
// 			h.cwnd[addr] = h.cwnd[addr] + 1
// 			h.cAvoidanceAcc[addr] = 0 // Reset accumulator when leaving slow start
// 		} else {
// 			// Congestion avoidance
// 			accu := h.cAvoidanceAcc[addr]
// 			accu++

// 			if accu >= cwnd {
// 				h.cwnd[addr] = h.cwnd[addr] + 1
// 				h.cAvoidanceAcc[addr] = 0 // TODO accu = 0 + test
// 			}

// 			h.cAvoidanceAcc[addr] = accu
// 		}
// 	}
// }
