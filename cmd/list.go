package cmd

import "bjoernblessin.de/chatprotogol/connection"

func HandleList(args []string) {
	if len(args) < 0 {
		println("Usage: list")
		return
	}

	routingTable := connection.GetRoutingTable()
	if len(routingTable.Entries) == 0 {
		println("No entries in the routing table.")
		return
	}

	println("Routing Table:")
	for addrPort, entry := range routingTable.Entries {
		println(addrPort.String(), "-> Hop Count:", entry.HopCount, "Next Hop:", entry.NextHop.String())
	}
}
