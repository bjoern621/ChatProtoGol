package cmd

import (
	"fmt"
	"net/netip"
	"time"

	"bjoernblessin.de/chatprotogol/connection"
	"bjoernblessin.de/chatprotogol/pkt"
)

var running bool
var lastChunkPktNum [4]byte
var peerIP netip.Addr

// HandleInfiniteMsg sends an infinite stream of messages to the specified IPv4 address.
func HandleInfiniteMsg(args []string) {
	if running {
		running = false

		payload := []byte(lastChunkPktNum[:])
		packet := connection.BuildSequencedPacket(pkt.MsgTypeFinish, payload, peerIP)

		err := connection.SendReliableRoutedPacket(packet)
		if err != nil {
			fmt.Printf("Failed to send finish message to %s: %v\n", peerIP, err)
		}

		fmt.Println("Infinite message sending stopped.")
		return
	}

	if len(args) < 1 {
		fmt.Println("Usage: infmsg <IPv4 address>")
		return
	}

	peerIP, err := netip.ParseAddr(args[0])
	if err != nil || !peerIP.Is4() {
		fmt.Printf("Invalid IPv4 address: %s\n", args[0])
		return
	}

	fmt.Printf("Sending infinite messages to %s. Cancel by using 'infmsg' again!\n", peerIP)
	time.Sleep(3 * time.Second)

	running = true

	go sendLoop(peerIP)
}

func sendLoop(peerIP netip.Addr) {
	for running {
		packet := connection.BuildSequencedPacket(pkt.MsgTypeChatMessage, []byte("testtesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttestesttestfjseofjsefjseofesijfddcawm8dcaw8u9cmd8u9aw8um9c0u89ac8u9mm89u0m89u0ca3m908uac3m0u980am8u93c098uaracm389ruu8a90m3rdu8md3radum89d3aru890da3ru89d03radmd8ur3aud38aru8d039arcu8d093arcmu8d93arcu8d9ßr3amud8ß3rau8dß3r9a8ußd3r9adduß83ra9ddu38ra9cdd3u8ra9cdd3ur8a9cd8d3uracdd38ur9ca ddu38r9 cdu38r9 aca8d3u9r a8u9d3ar c8uda93r c8u9d3arcdud839racud83r9acdß3u8r9acdd8u3ßr9ac8ud39ßra cd8u3d9rßac89ud3r acdu8d93 aru893ad r98 3adra89dah3pr98ahd3rpa8har3dh89 0rca890arc3w90h8 cr3a098hw ac9r38h a9c8rh3 9cah8r3 ch8ar3 9ahr83 9cah8r3 h8ca3r 9ch083ra m9chr830a mhc9r308aa8u39rcmwmu839racwmu8r3c9waum80cr93wu8mcr390wam80uc39rwm08u9r3cw09u8r3cw90u8cr3w09uc8r3wmcu98r30wuc8r3w9uc89r3ßwcmu89ßr3wcßmu839rwßcmu98r3wßcmu89r3wcßm8u9r3wcßm8u93rwmcu8ß93rwmcu83r9wc83r9wacmu8093awrmc8u093rwa0m98cu3rwamc0u93r8wcm0u89r3w0cm9u8r3w089cumr30uc89m3rwc0u893rwcr3aw,iß90cra3w,ß90ic3rwa,ß9i0c3rw9i0ac3rwa,ß90icr3wa9i0cr3wß,09icr3waß,90ic3rwa,09icr3w,09icr3wa,09ir3w,9i0cr3w,9i0cr3w,09icr3w,c09ir3wc09i3rc,039irwc,ßi9r0r39i,93crw,i93c"), peerIP)
		for running {
			err := connection.SendReliableRoutedPacket(packet)
			if err == nil {
				lastChunkPktNum = packet.Header.PktNum
				break // sent successfully, move to next packet
			}

			time.Sleep(100 * time.Millisecond)
		}
	}
}
