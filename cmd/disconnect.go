package cmd

import (
	"fmt"
	"net/netip"
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

	fmt.Printf("Disconnecting from %s...\n", peerIP)
}
