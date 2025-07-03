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
	senderWindows := outSequencing.GetSenderWindows()

	if len(senderWindows) == 0 {
		fmt.Println("No active peer connections with sender windows.")
		return
	}

	fmt.Println("Open Outgoing ACKs:")
	for peerAddr, windowSize := range senderWindows {
		pktNums, hasAcks := openAcks[peerAddr]
		if !hasAcks {
			// To make output cleaner, show an empty slice if no acks are open
			pktNums = []uint32{}
		}
		fmt.Printf("  %s -> Window: %d, Open ACKs: %v\n", peerAddr, windowSize, pktNums)
	}
}
