package common

const INITIAL_TTL = 30              // TTL for a new packet
const MAX_PAYLOAD_SIZE_BYTES = 1200 // MTU in bytes after subtracting ALL headers (IP, UDP, ChatProto)
const ACK_TIMEOUT_SECONDS = 5
const RETRIES_PER_PACKET = 2
const TEAM_ID = 0x2
const UDP_BUFFER_SIZE_BYTES = 1500    // Number of bytes to read from socket per packet (1500 is common MTU size for Ethernet)
const RECEIVE_BUFFER_SIZE = 1000      // Size of sequencing buffer per peer
const SOCKET_RECEIVE_BUFFER_SIZE = 10 // Number of packets to buffer in the socket before dropping them
