package cmd

import (
	"fmt"
	"strings"

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
	congestionWindows := outSequencing.GetCongestionWindows()
	thresholds := outSequencing.GetSlowStartThresholds()

	if len(congestionWindows) == 0 {
		fmt.Println("No active peer connections.")
		return
	}

	fmt.Println("Congestion Control Status:")
	for peerAddr, windowSize := range congestionWindows {
		ackInfos, hasAcks := openAcks[peerAddr]
		var ackStrings []string
		if hasAcks {
			for _, ack := range ackInfos {
				ackStrings = append(ackStrings, fmt.Sprintf("%d(timer: %s)", ack.PktNum, ack.TimerStatus))
			}
		}

		// Get the threshold, defaulting to 0 if not yet set for the peer
		threshold := thresholds[peerAddr]

		fmt.Printf("  %s -> Cwnd: %d, ssthresh: %d, Open ACKs: [%s]\n", peerAddr, windowSize, threshold, strings.Join(ackStrings, ", "))
	}
}
