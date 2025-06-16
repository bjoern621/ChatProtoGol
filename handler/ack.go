package handler

import (
	"net/netip"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/sock"
	"bjoernblessin.de/chatprotogol/util/assert"
	"bjoernblessin.de/chatprotogol/util/logger"
	"bjoernblessin.de/chatprotogol/util/observer"
)

type packetIdentifier struct {
	PeerAddr [4]byte
	PktNum   [4]byte
}

// ackObservers is an observable that emits when an ACK packet is received.
var ackObservers = make(map[packetIdentifier]*observer.Observable[any])

// SubscribeToReceivedAck subscribes to the observable for a specific packet.
// The channel will once receive a notification when the ACK for the given packet is received.
func SubscribeToReceivedAck(packet *pkt.Packet) chan any {
	id := packetIdentifier{
		PeerAddr: packet.Header.DestAddr,
		PktNum:   packet.Header.PktNum,
	}

	if _, exists := ackObservers[id]; !exists {
		ackObservers[id] = observer.NewObservable[any](1)
		assert.Assert(len(ackObservers) <= 256, "Too many ACK listeners registered, max is 256")
	}

	return ackObservers[id].SubscribeOnce()
}

func handleAck(packet *pkt.Packet, socket sock.Socket, outSequencing *sequencing.OutgoingPktNumHandler) {
	logger.Infof("ACK RECEIVED %v %d", packet.Header.SourceAddr, packet.Header.PktNum)

	destAddr := netip.AddrFrom4(packet.Header.DestAddr)
	if destAddr != socket.MustGetLocalAddress().Addr() {
		// The acknowledgment is for another peer, forward it

		destPeer, found := connection.GetPeer(destAddr)
		if !found {
			logger.Warnf("No peer found for destination address %s, can't forward", destAddr)
			return
		}

		destPeer.Forward(packet)
		return
	}

	// The acknowledgment is for us, remove the open acknowledgment

	sourceAddr := netip.AddrFrom4([4]byte(packet.Header.SourceAddr))
	outSequencing.RemoveOpenAck(sourceAddr, packet.Header.PktNum)

	packetId := packetIdentifier{
		PeerAddr: packet.Header.SourceAddr,
		PktNum:   packet.Header.PktNum,
	}

	if observable, exists := ackObservers[packetId]; exists {
		observable.NotifyObservers(nil)
		delete(ackObservers, packetId)
	}
}
