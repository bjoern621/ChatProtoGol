package connection

import (
	"fmt"
	"time"

	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/util/assert"
)

var sequenceNumbers = make(map[*Peer]uint32)
var openAcks = make(map[*Peer]map[[4]byte]*OpenAck)

type OpenAck struct {
	timer   *time.Timer
	packet  *pkt.Packet
	retries int
}

func ClearSequenceNumbers(peer *Peer) {
	delete(sequenceNumbers, peer)

	if acks, exists := openAcks[peer]; exists {
		for seqNum, ack := range acks {
			ack.timer.Stop()
			delete(openAcks[peer], seqNum)
		}
	}
}

// getNextSequenceNumber returns the next sequence number for the given address.
func getNextSequenceNumber(peer *Peer) [4]byte {
	seqNum, exists := sequenceNumbers[peer]
	if !exists {
		seqNum = 0
	}

	sequenceNumbers[peer] = seqNum + 1

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
	fmt.Printf("ACK timeout for peer %s with sequence number %v\n", peer.Address, packet.Header.SeqNum)

	nextHop, found := GetNextHop(peer.Address)
	if !found {
		fmt.Printf("Peer %s is no longer reachable, removing open acknowledgment for sequence number %v", peer.Address, packet.Header.SeqNum)
		return // Peer no longer reachable (e.g., disconnected)
	}

	peer.sendPacketTo(nextHop, packet)

	openAck, exists := openAcks[peer][packet.Header.SeqNum] // TODO
	assert.Assert(exists, "No open acknowledgment found for peer %s with sequence number %v", peer.Address, packet.Header.SeqNum)

	openAck.retries--
	if openAck.retries <= 0 {
		delete(openAcks[peer], packet.Header.SeqNum)
		return // No more retries left, remove the open acknowledgment
	}

	openAck.timer.Reset(time.Second * common.ACK_TIMEOUT_SECONDS)
}

// RemoveOpenAck removes a sequence number from the open acknowledgments for the given peer.
// If the sequence number does not exist, it does nothing.
func RemoveOpenAck(peer *Peer, seqNum [4]byte) {
	openAck, exists := openAcks[peer][seqNum]
	if !exists {
		return
	}

	openAck.timer.Stop()
	delete(openAcks[peer], seqNum)
}
