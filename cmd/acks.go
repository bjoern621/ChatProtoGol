package cmd

import (
	"fmt"

	"bjoernblessin.de/chatprotogol/util/logger"
)

// HandleListAcks displays all open outgoing acknowledgments.
func HandleListAcks(args []string) {
	if len(args) != 0 {
		logger.Warnf("Usage: acks")
		return
	}

	if outSequencing == nil {
		logger.Warnf("Outgoing sequencing is not initialized.")
		return
	}

	openAcks := outSequencing.GetOpenAcks()

	if len(openAcks) == 0 {
		fmt.Println("No open outgoing ACKs.")
		return
	}

	fmt.Println("Open Outgoing ACKs:")
	for peerAddr, pktNums := range openAcks {
		fmt.Printf("  %s -> %v\n", peerAddr, pktNums)
	}
}
