package connection

import (
	"net/netip"
	"time"

	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/util/assert"
	"bjoernblessin.de/chatprotogol/util/logger"
)

var sequenceNumbers = make(map[netip.Addr]uint32)
var openAcks = make(map[netip.Addr]map[[4]byte]*OpenAck)

type OpenAck struct {
	timer   *time.Timer
	retries int
}

// ClearSequenceNumbers clears the current sequence number and open acknowledgments for the given peer.
func ClearSequenceNumbers(peerAddr netip.Addr) {
	delete(sequenceNumbers, peerAddr)

	if acks, exists := openAcks[peerAddr]; exists {
		for seqNum, ack := range acks {
			ack.timer.Stop()
			delete(openAcks[peerAddr], seqNum)
		}
	}
}

// getNextSequenceNumber returns the next sequence number for the given address.
func getNextSequenceNumber(peerAddr netip.Addr) [4]byte {
	seqNum, exists := sequenceNumbers[peerAddr]
	if !exists {
		seqNum = 0
	}

	sequenceNumbers[peerAddr] = seqNum + 1

	return [4]byte{
		byte(seqNum >> 24),
		byte(seqNum >> 16),
		byte(seqNum >> 8),
		byte(seqNum),
	}
}

// addOpenAck adds a sequence number to the open acknowledgments for the given peer and starts a new timeout timer.
// After the timeout, it will call the provided resend function to resend the packet.
func addOpenAck(peerAddr netip.Addr, seqNum [4]byte, resendFunc func()) {
	if _, exists := openAcks[peerAddr]; !exists {
		openAcks[peerAddr] = map[[4]byte]*OpenAck{}
	}

	openAck := &OpenAck{
		timer:   time.AfterFunc(time.Second*common.ACK_TIMEOUT_SECONDS, func() { handleAckTimeout(peerAddr, seqNum, resendFunc) }),
		retries: common.RETRIES_PER_PACKET,
	}

	openAcks[peerAddr][seqNum] = openAck
}

// handleAckTimeout is called when an acknowledgment timeout occurs.
func handleAckTimeout(peerAddr netip.Addr, seqNum [4]byte, resendFunc func()) {
	logger.Warnf("ACK timeout for peer %s with sequence number %v\n", peerAddr, seqNum)

	resendFunc()

	// nextHop, found := GetNextHop(peerAddr)
	// if !found {
	// 	logger.Infof("Peer %s is no longer reachable, removing open acknowledgment for sequence number %v", peerAddr, packet.Header.SeqNum)
	// 	return // Peer no longer reachable (e.g., disconnected)
	// }

	// peer.sendPacketTo(nextHop, packet)

	openAck, exists := openAcks[peerAddr][seqNum]
	assert.Assert(exists, "No open acknowledgment found for peer %s with sequence number %v", peerAddr, seqNum) // TODO may fail?

	openAck.retries--
	if openAck.retries <= 0 {
		delete(openAcks[peerAddr], seqNum)
		return // No more retries left, remove the open acknowledgment
	}

	openAck.timer.Reset(time.Second * common.ACK_TIMEOUT_SECONDS)
}

// RemoveOpenAck removes a sequence number from the open acknowledgments for the given peer.
// If the sequence number does not exist, it does nothing.
func RemoveOpenAck(peerAddr netip.Addr, seqNum [4]byte) {
	openAck, exists := openAcks[peerAddr][seqNum]
	if !exists {
		return
	}

	openAck.timer.Stop()
	delete(openAcks[peerAddr], seqNum)
}
