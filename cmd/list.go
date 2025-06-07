package cmd

import (
	"fmt"

	"bjoernblessin.de/chatprotogol/connection"
)

func HandleList(args []string) {
	if len(args) < 0 {
		fmt.Printf("Usage: list")
		return
	}

	routingTable := connection.GetRoutingTable()
	if len(routingTable.Entries) == 0 {
		fmt.Printf("No entries in the routing table.\n")
		return
	}

	fmt.Printf("Routing Table:\n")
	for addrPort, entry := range routingTable.Entries {
		fmt.Printf("  %s -> Hop Count: %d, Next Hop: %s\n", addrPort, entry.HopCount, entry.NextHop)
	}
}
