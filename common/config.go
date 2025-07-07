package common

import (
	"math"
	"os"
	"path/filepath"
	"time"
)

const INITIAL_TTL = 30              // TTL for a new packet
const MAX_PAYLOAD_SIZE_BYTES = 1200 // MTU in bytes after subtracting ChatProtocol header: 1484
const ACK_TIMEOUT_DURATION = time.Second * 2
const RETRIES_PER_PACKET = 10 // Number of times to retry sending a packet before giving up; -1 means infinite retries
const TEAM_ID = 0x2
const UDP_BUFFER_SIZE_BYTES = 1500                  // Number of bytes to read from socket per packet (1500 is common MTU size for Ethernet); incoming packets larger than this will be dropped
const RECEIVER_WINDOW = math.MaxInt64               // Size of sequencing buffer per peer
const SOCKET_RECEIVE_BUFFER_SIZE = 500              // Number of packets to buffer in the receiving socket channel before dropping them
const PACKET_HANDLER_GOROUTINES = 100               // Number of goroutines to handle incoming packets concurrently
const CWND_FULL_RETRY_DELAY = time.Millisecond * 50 // Duration before retrying to send a file / msg chunk after sender congestion overflow
const INITIAL_CWND = 10                             // Size of the initial congestion window for new connections; this is the number of packets that can be sent before waiting for an acknowledgment, modified dynamically per peer based on ACKs received
const IGNORE_CWND = false                           // If true, the congestion window will not limit the number of packets sent

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
