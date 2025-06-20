package cmd

import (
	"fmt"
)

func HandleList(args []string) {
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
