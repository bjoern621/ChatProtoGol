package cmd

import (
	"net/netip"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/util/assert"
)

// HandleDisconnect processes the "disconnect" command to disconnect from a specified peer.
// It sends a disconnect message to the peer and removes it from the managed peers.
// It sends an updated routing table to all remaining neighbors.
func HandleDisconnect(args []string) {
	if len(args) < 1 {
		println("Usage: disconnect <IPv4 address> Example: disconnect 10.10.10.2")
		return
	}

	peerIP, err := netip.ParseAddr(args[0])
	if err != nil {
		println("Invalid IPv4 address:", args[0])
		return
	}

	peer, found := connection.GetPeer(peerIP)
	if !found {
		println("Peer not found:", args[0])
		return
	}

	err = peer.SendNew(pkt.MsgTypeDisconnect, true, nil)
	if err != nil {
		println("Failed to send disconnect message:", err)
		return
	}

	nextHop, found := connection.GetNextHop(peerIP)
	assert.Assert(found, "Next hop for peer %s not found but should be available because the peer was found", peerIP)

	peer.Delete()
	connection.RemoveRoutingEntriesWithNextHop(nextHop)
	connection.ClearSequenceNumbers(peer)

	connection.SendCurrentRoutingTable(connection.GetAllNeighbors())
}
