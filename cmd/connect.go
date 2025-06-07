package cmd

import (
	"fmt"
	"net/netip"
	"strconv"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
)

// HandleConnect processes the "connect" command to establish a connection to a specified IP address and port.
func HandleConnect(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: connect <IP address> <port> Example: connect 10.0.0.2 8080")
		return
	}

	peerIP, err := netip.ParseAddr(args[0])
	if err != nil {
		fmt.Printf("Invalid IP address: %s\n", args[0])
		return
	}

	port, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Printf("Invalid port number: %s\n", args[1])
		return
	}

	if !peerIP.Is4() {
		fmt.Printf("The provided IP address is not a valid IPv4 address: %s\n", args[0])
		return
	}

	// Add the peer to the routing table
	peerAddrPort := netip.AddrPortFrom(peerIP, uint16(port))
	routeEntry := connection.RouteEntry{
		NextHop:  peerAddrPort,
		HopCount: 1,
	}
	connection.Update(connection.RoutingTable{
		Entries: map[netip.AddrPort]connection.RouteEntry{
			peerAddrPort: routeEntry,
		},
	}, peerAddrPort)

	peer := connection.NewPeer(peerAddrPort)
	err = peer.Send(pkt.MsgTypeConnect, true, nil)
	if err != nil {
		fmt.Printf("Failed to send connect message: %v\n", err)
		return
	}
}
