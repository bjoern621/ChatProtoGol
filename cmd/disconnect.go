package cmd

import (
	"fmt"
	"net/netip"

	"bjoernblessin.de/chatprotogol/connection"
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

	doneChan, err := disconnectFrom(addr)
	if err != nil {
		fmt.Printf("Error disconnecting from %s: %v\n", addr, err)
		return
	}

	success := <-doneChan
	fmt.Printf("Disconnected from %s\n", addr)

	if !success {
		fmt.Printf("No ACK received from  %s\n", addr)
		fmt.Printf("Disconnected from %s anyway, but the other side might not be aware of it.\n", addr)
	}
}

// disconnectFrom sends a disconnect message to the specified address and handles the complete disconnect.
// It returns a channel that will receive either true or false once, indicating whether the disconnect was successful.
// After disconnectFrom the address might be still reachable through other connections, but the direct connection is closed.
// Will close the connection even if the ACK is not received, but will signal failure (false) if the ACK is not received.
func disconnectFrom(addr netip.Addr) (<-chan bool, error) {
	doneChan := make(chan bool, 1)

	isNeighbor, _ := router.IsNeighbor(addr)
	if !isNeighbor {
		return nil, fmt.Errorf("not connected to %s", addr)
	}

	packet := connection.BuildSequencedPacket(pkt.MsgTypeDisconnect, nil, addr)

	ackChan, err := connection.SendReliableRoutedPacket(packet)
	if err != nil {
		return nil, err
	}

	go func() {
		success := <-ackChan

		unreachableHosts := router.RemoveNeighbor(addr)
		connection.ClearUnreachableHosts(unreachableHosts)

		localAddr := socket.MustGetLocalAddress().Addr()
		localLSA, exists := router.GetLSA(localAddr)
		assert.Assert(exists, "LSA should exist for the local address")
		connection.FloodLSA(localAddr, localLSA)

		doneChan <- success
	}()

	return doneChan, nil
}
