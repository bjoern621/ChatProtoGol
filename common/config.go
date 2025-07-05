package common

import (
	"math"
	"os"
	"path/filepath"
	"time"
)

const INITIAL_TTL = 30              // TTL for a new packet
const MAX_PAYLOAD_SIZE_BYTES = 1484 // MTU in bytes after subtracting ChatProtocol header: 1484
const ACK_TIMEOUT_DURATION = time.Millisecond * 100
const RETRIES_PER_PACKET = 10 // Number of times to retry sending a packet before giving up; -1 means infinite retries
const TEAM_ID = 0x2
const UDP_BUFFER_SIZE_BYTES = 1500                      // Number of bytes to read from socket per packet (1500 is common MTU size for Ethernet); incoming packets larger than this will be dropped
const RECEIVER_WINDOW = math.MaxInt64                   // Size of sequencing buffer per peer
const SOCKET_RECEIVE_BUFFER_SIZE = 500                  // Number of packets to buffer in the receiving socket channel before dropping them
const PACKET_HANDLER_GOROUTINES = 100                   // Number of goroutines to handle incoming packets concurrently
const INITIAL_SENDER_WINDOW = 100                       // Size of sequencing buffer for sending packets per peer; this is the initial number of packets that can be sent without waiting for an acknowledgment, modified dynamically based on ACKs received
const FILE_TRANSFER_RETRY_DELAY = time.Millisecond * 50 // Duration before retrying to send a file chunk after sender window overflow
const WINDOW_DISCARD_THRESHOLD = 3                      // Number of packets in the receiver window after which the reeiver will discard old packets to make room for new ones
var RECEIVED_FILES_DIR string

func init() {
	const subdirectory = "chatprotogol_received_files"
	dir, err := os.UserHomeDir()
	if err != nil {
		RECEIVED_FILES_DIR = string(os.PathSeparator) + subdirectory
	} else {
		RECEIVED_FILES_DIR = filepath.Join(dir, subdirectory)
	}
}
