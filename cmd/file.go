package cmd

import (
	"fmt"
	"io"
	"net/netip"
	"os"
	"sync"
	"time"

	"bjoernblessin.de/chatprotogol/common"
	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/util/logger"
	"github.com/schollz/progressbar/v3"
)

func HandleSendFile(args []string) {
	if len(args) < 2 {
		println("Usage: file <IPv4 address> <file path>")
		return
	}

	peerIP, err := netip.ParseAddr(args[0])
	if err != nil || !peerIP.Is4() {
		println("Invalid IPv4 address:", args[0])
		return
	}

	filePath := args[1]
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		fmt.Printf("Failed to get file info for %s: %v\n", args[1], err)
		return
	}

	if fileInfo.IsDir() {
		fmt.Printf("The specified path %s is a directory, not a file.\n", args[1])
		return
	}

	packet := connection.BuildSequencedPacket(pkt.MsgTypeFileTransfer, []byte(fileInfo.Name()), peerIP)
	_, err = connection.SendReliableRoutedPacket(packet)
	if err != nil {
		logger.Warnf("Failed to send metadata packet to %s: %v, cancelling file transfer\n", peerIP, err)
		return
	}

	blocker := sequencing.GetSequenceBlocker(peerIP, pkt.MsgTypeFileTransfer)
	success := blocker.Block()
	if !success {
		fmt.Printf("Can't send file to %s: Another file is currently being sent.\n", peerIP)
		return
	}

	go sendFileChunks(peerIP, filePath, blocker)
}

func sendFileChunks(peerIP netip.Addr, filePath string, blocker *sequencing.SequenceBlocker) {
	defer blocker.Unblock()

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Failed to open file %s: %v\n", filePath, err)
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		fmt.Printf("Failed to get file info for %s: %v\n", filePath, err)
		return
	}

	bar := progressbar.NewOptions(int(fileInfo.Size()),
		progressbar.OptionSetDescription(fmt.Sprintf("Sending %s", fileInfo.Name())),
		progressbar.OptionShowBytes(true),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionOnCompletion(func() {
			fmt.Printf("\n")
		}),
	)

	wg := &sync.WaitGroup{} // Used to wait for file chuck ACKs and the FIN message ACK
	var lastChunkPktNum [4]byte

	buffer := make([]byte, common.MAX_PAYLOAD_SIZE_BYTES)
	for {
		n, err := file.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}

			fmt.Printf("Failed to read file %s: %v\n", file.Name(), err)
		}

		packet := connection.BuildSequencedPacket(pkt.MsgTypeFileTransfer, buffer[:n], peerIP)

		ackChan, err := connection.SendReliableRoutedPacket(packet)
		for err != nil {
			time.Sleep(common.FILE_TRANSFER_RETRY_DELAY)
			logger.Debugf("Failed to send file chunk %v to %s, retrying: %v", packet.Header.PktNum, peerIP, err)
			ackChan, err = connection.SendReliableRoutedPacket(packet)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			<-ackChan
			// We ignore the success of the ACK to avoid blocking the send process. The receiver might get a faulty file.
			bar.Add(n)
		}()

		lastChunkPktNum = packet.Header.PktNum
	}

	// Send the FIN message after all chunks have been sent and acknowledged
	wg.Wait()

	payload := []byte(lastChunkPktNum[:])
	packet := connection.BuildSequencedPacket(pkt.MsgTypeFinish, payload, peerIP)

	ackChan, err := connection.SendReliableRoutedPacket(packet)
	for err != nil {
		time.Sleep(common.FILE_TRANSFER_RETRY_DELAY)
		logger.Debugf("Failed to send finish message to %s: %v\n", peerIP, err)
		ackChan, err = connection.SendReliableRoutedPacket(packet)
	}

	wg.Add(1)
	go func() {
		<-ackChan
		// We ignore the success of the ACK to avoid blocking the send process. The receiver might not be ready for a new message but we don't care.
	}()
}
