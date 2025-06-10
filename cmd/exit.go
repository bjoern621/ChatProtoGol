package cmd

import (
	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
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

		connection.SendCurrentRoutingTable(connection.GetAllNeighbors())
	}
}
