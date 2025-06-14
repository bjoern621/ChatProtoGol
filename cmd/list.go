package cmd

import (
	"fmt"
)

func HandleList(args []string) {
	if len(args) < 0 {
		fmt.Printf("Usage: list")
		return
	}

	routingTable := router.GetRoutingTable()
	if len(routingTable) == 0 {
		fmt.Printf("No entries in the routing table.\n")
		return
	}

	fmt.Printf("Routing Table:\n")
	for addrPort, nextHop := range routingTable {
		fmt.Printf("  %s -> Next Hop: %s\n", addrPort, nextHop)
	}
}
