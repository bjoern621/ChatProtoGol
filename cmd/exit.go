package cmd

import (
	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/util/assert"
)

func HandleExit(args []string) {
	neighbors := connection.GetAllNeighbors()

	for _, neighbor := range neighbors {
		err := neighbor.SendNew(pkt.MsgTypeDisconnect, true, nil)
		if err != nil {
			println("Failed to send disconnect message while exiting:", err)
			continue
		}

		neighbor.Delete()
		nextHop, found := connection.GetNextHop(neighbor.Address)
		assert.Assert(found)
		connection.RemoveRoutingEntriesWithNextHop(nextHop)
		connection.ClearSequenceNumbers(neighbor)

		connection.SendCurrentRoutingTable(connection.GetAllNeighbors())
	}
}
