package cmd

import (
	"fmt"
	"net/netip"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/handler"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/util/assert"
)

func HandleDisconnect(args []string) {
	if len(args) < 1 {
		println("Usage: disconnect <IPv4 address> Example: disconnect 10.10.10.2")
		return
	}

	addr, err := netip.ParseAddr(args[0])
	if err != nil {
		println("Invalid IPv4 address:", args[0])
		return
	}

	isNeighbor, _ := router.IsNeighbor(addr)
	if !isNeighbor {
		fmt.Printf("Not connected to %s\n", addr)
		return
	}

	packet := connection.BuildSequencedPacket(pkt.MsgTypeDisconnect, true, nil, addr)

	go func() {
		for range handler.SubscribeToReceivedAck(packet) {
			unreachableHosts := router.RemoveNeighbor(addr)
			connection.ClearUnreachableHosts(unreachableHosts)

			localAddr := socket.MustGetLocalAddress().Addr()
			localLSA, exists := router.GetLSA(localAddr)
			assert.Assert(exists, "Local LSA should exist for the local address")
			connection.FloodLSA(localAddr, localLSA)

			fmt.Printf("Disconnected from %s\n", addr)
			break
		}
	}()

	err = connection.SendReliableRoutedPacket(packet)
	if err != nil {
		fmt.Printf("Failed to send disconnect message: %v\n", err)
		return
	}
}
