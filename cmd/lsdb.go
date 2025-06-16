package cmd

import (
	"fmt"

	"bjoernblessin.de/chatprotogol/util/logger"
)

func HandleListDatabase(args []string) {
	if len(args) != 0 {
		logger.Warnf("Usage: lsdb")
		return
	}

	if router == nil {
		logger.Warnf("Router is not initialized.")
		return
	}

	fmt.Println("Local Link State Database:")
	for _, lsaAddr := range router.GetAvailableLSAs() {
		lsa, exists := router.GetLSA(lsaAddr)
		if !exists {
			fmt.Printf("  %s -> (not found)\n", lsaAddr)
			continue
		}
		fmt.Printf("  %s -> %+v\n", lsaAddr, lsa)
	}
}
