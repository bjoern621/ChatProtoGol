package main

import (
	"fmt"
	"log"
	"net"

	"bjoernblessin.de/chatprotogol/cmd"
	"bjoernblessin.de/chatprotogol/cmd/inputreader"
	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/handler"
	"bjoernblessin.de/chatprotogol/routing"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/sock"
	"bjoernblessin.de/chatprotogol/util/logger"
)

func main() {
	log.Println("Running...")

	udpSocket := sock.NewUDPSocket()

	inSequencing := sequencing.NewIncomingPktNumHandler(udpSocket)
	outSequencing := sequencing.NewOutgoingPktNumHandler(common.INITIAL_SENDER_WINDOW)

	router := routing.NewRouter(udpSocket)

	cmd.SetGlobalVars(udpSocket, router, outSequencing)

	reader := inputreader.NewInputReader(udpSocket)

	reader.AddHandler("con", cmd.HandleConnect)
	reader.AddHandler("dis", cmd.HandleDisconnect)
	reader.AddHandler("msg", cmd.HandleSend)
	reader.AddHandler("file", cmd.HandleSendFile)
	reader.AddHandler("init", cmd.HandleInit)
	reader.AddHandler("ls", cmd.HandleList)
	reader.AddHandler("exit", cmd.HandleExit)
	reader.AddHandler("lsdb", cmd.HandleListDatabase)
	reader.AddHandler("infmsg", cmd.HandleInfiniteMsg)
	reader.AddHandler("i", cmd.HandleInit)
	reader.AddHandler("acks", cmd.HandleListAcks)
	reader.AddHandler("loglvl", cmd.HandleLogLevel)

	handler := handler.NewPacketHandler(udpSocket, router, inSequencing, outSequencing)
	go handler.ListenToPackets()

	connection.SetGlobalVars(udpSocket, router, inSequencing, outSequencing)

	localAddr, err := udpSocket.Open(net.IPv4(127, 0, 0, 1))
	if err != nil {
		logger.Errorf("Failed to open UDP socket: %v", err)
		return
	}
	fmt.Printf("Listening on %s:%d\n", localAddr.IP, localAddr.Port)

	printAvailableNetworkAddresses()

	reader.InputLoop()
}

func printAvailableNetworkAddresses() {
	inter, err := net.Interfaces()
	if err != nil {
		logger.Warnf("Failed to get network interfaces: %v", err)
		return
	}

	fmt.Println("Available network interfaces:")

	for _, iface := range inter {
		if iface.Flags&net.FlagUp == 0 {
			continue // Skip down interfaces
		}
		addrs, err2 := iface.Addrs()
		if err2 != nil {
			logger.Warnf("Failed to get addresses for interface %s: %v", iface.Name, err2)
			continue
		}

		for _, addr := range addrs {
			ip, ok := addr.(*net.IPNet)
			if !ok {
				continue // Skip non-IP addresses
			}

			if ip.IP.To4() == nil {
				continue // Skip non-IPv4 addresses
			}

			fmt.Printf("  Interface: %s, Address: %s\n", iface.Name, ip.IP)
		}
	}
}
