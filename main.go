package main

import (
	"fmt"
	"log"

	"bjoernblessin.de/chatprotogol/cmd"
	"bjoernblessin.de/chatprotogol/inputreader"
	"bjoernblessin.de/chatprotogol/socket"
	"bjoernblessin.de/chatprotogol/util/logger"
)

func main() {
	log.Println("Running...")

	reader := inputreader.NewInputReader()

	reader.AddHandler("connect", cmd.HandleConnect)
	reader.AddHandler("disconnect", cmd.HandleDisconnect)
	reader.AddHandler("send", cmd.HandleSend)
	reader.AddHandler("sendfile", cmd.HandleSendFile)

	port, err := socket.Open()
	if err != nil {
		logger.Errorf("Failed to open UDP socket: %v", err)
	}

	fmt.Print("Listening on port: ", port, "\n")

	reader.InputLoop()
}
