package handler

import (
	"net"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/routing"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/skt"
	"bjoernblessin.de/chatprotogol/util/assert"
	"bjoernblessin.de/chatprotogol/util/logger"
)

// handleRoutingDuplicate checks if the packet is a duplicate.
// If it is, it sends an acknowledgment back to the sender.
// Returns true if the packet was handled (duplicate or errornous packet), false otherwise.
func handleRoutingDuplicate(packet *pkt.Packet, sourceAddr *net.UDPAddr, inSequencing *sequencing.IncomingPktNumHandler) (handled bool) {
	duplicate, dupErr := inSequencing.IsDuplicatePacket(packet)
	if dupErr != nil {
		return true
	} else if duplicate {
		peer, exists := connection.GetPeer(sourceAddr.AddrPort().Addr())
		assert.Assert(exists, "Duplicate packet should have a peer")
		peer.SendAcknowledgment(packet.Header.PktNum)
		return true
	}

	return false
}

// handleConnect processes a connection request from a peer.
// It adds the (maybe new) peer to the routing table with a hop count of 1.
// It sends an acknowledgment back to the sender.
// It sends the current routing table to all neighbors (including the new peer).
func handleConnect(packet *pkt.Packet, sourceAddr *net.UDPAddr, socket skt.Socket, router *routing.Router, inSequencing *sequencing.IncomingPktNumHandler) {
	handled := handleRoutingDuplicate(packet, sourceAddr, inSequencing)
	if handled {
		return
	}

	logger.Infof("CONN FROM %v %v", packet.Header.SourceAddr, packet.Header.PktNum)

	router.AddNeighbor(sourceAddr.AddrPort())
	router.RecalculateLocalLSA()
	router.BuildRoutingTable(socket)

	connection.SendAcknowledgment(sourceAddr.AddrPort().Addr(), packet.Header.PktNum)
}

// handleDisconnect processes a disconnect request from a peer.
// It sends an acknowledgment back to the sender.
// It removes all peers routed trough the disconnected peer from the routing table and clears their sequence numbers.
// It sends an updated routing table to all neighbors (excluding the disconnected peer).
func handleDisconnect(packet *pkt.Packet, sourceAddr *net.UDPAddr, inSequencing *sequencing.IncomingPktNumHandler) {
	handled := handleRoutingDuplicate(packet, sourceAddr, inSequencing)
	if handled {
		return
	}

	logger.Infof("DISCO FROM %v %v", packet.Header.SourceAddr, packet.Header.PktNum)

	if isNeighbor, _ := routing.IsNeighbor2(sourceAddr.AddrPort().Addr()); !isNeighbor {
		logger.Warnf("Received disconnect from non-neighbor peer %v", sourceAddr.AddrPort().Addr())
		return
	}

	peer, exists := connection.GetPeer(sourceAddr.AddrPort().Addr())
	if !exists {
		logger.Warnf("Received disconnect from unknown peer %v", sourceAddr.AddrPort().Addr())
		return
	}
	peer.SendAcknowledgment(packet.Header.PktNum)

	peer.Delete()

	connection.SendCurrentRoutingTable(connection.GetAllNeighbors())
}

// handleRoutingTableUpdate processes a routing table update packet from a peer.
// Send an acknowledgment back to the sender.
// Update the local routing table with the new information.
// Forward the routing table to other peers if necessary.
func handleRoutingTableUpdate(packet *pkt.Packet, sourceAddr *net.UDPAddr, inSequencing *sequencing.IncomingPktNumHandler) {
	handled := handleRoutingDuplicate(packet, sourceAddr, inSequencing)
	if handled {
		return
	}

	logger.Infof("ROUTING FROM %v %v", packet.Header.SourceAddr, packet.Header.PktNum)

	rt, err := routing.ParseRoutingTableFromPayload(packet.Payload, sourceAddr.AddrPort())
	if err != nil {
		logger.Warnf("Failed to parse routing table from payload: %v", err)
		return
	}

	routingTableChanged := routing.UpdateRoutingTable(rt, sourceAddr.AddrPort())

	peer, exists := connection.GetPeer(sourceAddr.AddrPort().Addr())
	if !exists {
		logger.Warnf("Received routing table update from unknown peer %v", sourceAddr.AddrPort().Addr())
		return
	}
	peer.SendAcknowledgment(packet.Header.PktNum)

	if routingTableChanged { // TODO resend update messages change
		neighbors := connection.GetAllNeighbors()
		delete(neighbors, sourceAddr.AddrPort().Addr()) // Exclude the sender from the update

		connection.SendCurrentRoutingTable(neighbors)
	}
}
