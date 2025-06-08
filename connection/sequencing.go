package connection

import (
	"fmt"
	"net/netip"
	"time"

	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/util/assert"
)

var sequenceNumbers = make(map[netip.Addr]uint32)
var openAcks = make(map[*Peer]map[[4]byte]*OpenAck)

type OpenAck struct {
	timer   *time.Timer
	packet  *pkt.Packet
	retries int
}

// getNextSequenceNumber returns the next sequence number for the given address.
func getNextSequenceNumber(addr netip.Addr) [4]byte {
	seqNum, exists := sequenceNumbers[addr]
	if !exists {
		seqNum = 0
	}

	sequenceNumbers[addr] = seqNum + 1

	return [4]byte{
		byte(seqNum >> 24),
		byte(seqNum >> 16),
		byte(seqNum >> 8),
		byte(seqNum),
	}
}

// addOpenAck adds a sequence number to the open acknowledgments for the given peer and starts a new timeout timer.
func addOpenAck(peer *Peer, packet *pkt.Packet) {
	if _, exists := openAcks[peer]; !exists {
		openAcks[peer] = map[[4]byte]*OpenAck{}
	}

	openAck := &OpenAck{
		timer:   time.AfterFunc(time.Second*common.ACK_TIMEOUT_SECONDS, func() { handleAckTimeout(peer, packet) }),
		packet:  packet,
		retries: common.RETRIES_PER_PACKET,
	}

	openAcks[peer][packet.Header.SeqNum] = openAck
}

// handleAckTimeout is called when an acknowledgment timeout occurs.
func handleAckTimeout(peer *Peer, packet *pkt.Packet) {
	fmt.Printf("ACK timeout for peer %s with sequence number %v\n", peer.address, packet.Header.SeqNum)

	nextHop, found := getNextHop(peer.address)
	if !found {
		fmt.Printf("Peer %s is no longer reachable, removing open acknowledgment for sequence number %v", peer.address, packet.Header.SeqNum)
		return // Peer no longer reachable (e.g., disconnected)
	}

	peer.sendPacketTo(nextHop, packet)

	openAck, exists := openAcks[peer][packet.Header.SeqNum]
	assert.Assert(exists, "No open acknowledgment found for peer %s with sequence number %v", peer.address, packet.Header.SeqNum)

	openAck.retries--
	if openAck.retries <= 0 {
		delete(openAcks[peer], packet.Header.SeqNum)
		return // No more retries left, remove the open acknowledgment
	}

	openAck.timer.Reset(time.Second * common.ACK_TIMEOUT_SECONDS)
}

// RemoveOpenAck removes a sequence number from the open acknowledgments for the given peer.
func RemoveOpenAck(peer *Peer, seqNum [4]byte) {
	openAck, exists := openAcks[peer][seqNum]
	assert.Assert(exists, "No open acknowledgment found for address %s with sequence number %v", peer.address, seqNum)

	openAck.timer.Stop()
	delete(openAcks[peer], seqNum)
}
