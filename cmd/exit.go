package cmd

import (
	"fmt"
)

func HandleExit(args []string) {
	println("Exiting...")

	disconnectAll()
}

func disconnectAll() {
	for addr := range router.GetNeighbors() {
		doneChan, err := disconnectFrom(addr)
		if err != nil {
			fmt.Printf("Error disconnecting from %s: %v\n", addr, err)
			continue
		}

		<-doneChan
		fmt.Printf("Disconnected from %s\n", addr)
	}
}
