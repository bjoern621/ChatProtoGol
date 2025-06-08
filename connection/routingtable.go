package connection

import (
	"errors"
	"net/netip"

	"bjoernblessin.de/chatprotogol/socket"
)

type RouteEntry struct {
	HopCount int
	NextHop  netip.AddrPort // The IPv4 address and port of the next hop host
}

type RoutingTable struct {
	Entries map[netip.Addr]RouteEntry // Maps IPv4 addresses to route entries
}

var routingTable = RoutingTable{
	Entries: make(map[netip.Addr]RouteEntry),
}

// FormatRoutingTableForPayload formats the routing table for inclusion in a Routing Table Update Message.
// Format:
//
//	+--------+--------+--------+--------+--------+
//	|                                   |        |
//	|   IPv4 Address of Destination     |  Hop   |
//	|          (32 bits)                | Count  |
//	|                                   |(8 bits)|
//	+--------+--------+--------+--------+--------+
//	|                                   |        |
//	|   IPv4 Address of Destination     |  Hop   |
//	|          (32 bits)                | Count  |
//	|                                   |(8 bits)|
//	+--------+--------+--------+--------+--------+
//	|                                            |
//	|                 ...                        |
//	|                                            |
//	+--------+--------+--------+--------+--------+
func FormatRoutingTableForPayload() []byte {
	payload := make([]byte, 0, len(routingTable.Entries)*5) // 4 bytes for IPv4 address + 1 byte for hop count

	for destinationIP, entry := range routingTable.Entries {
		ipv4Bytes := destinationIP.As4()
		payload = append(payload, ipv4Bytes[:]...)
		payload = append(payload, byte(entry.HopCount))
	}

	return payload
}

// ParseRoutingTableFromPayload is the reverse of FormatRoutingTableForPayload.
func ParseRoutingTableFromPayload(payload []byte, receivedFrom netip.AddrPort) (RoutingTable, error) {
	if len(payload)%5 != 0 {
		return RoutingTable{}, errors.New("payload length must be a multiple of 5 bytes")
	}

	routingTable := RoutingTable{
		Entries: make(map[netip.Addr]RouteEntry),
	}

	for i := 0; i < len(payload); i += 5 {
		ipv4Bytes := payload[i : i+4]
		hopCount := payload[i+4]

		destinationIP, ok := netip.AddrFromSlice(ipv4Bytes)
		if !ok || !destinationIP.Is4() {
			return RoutingTable{}, errors.New("invalid IPv4 address in payload")
		}

		routingTable.Entries[destinationIP] = RouteEntry{
			HopCount: int(hopCount),
			NextHop:  receivedFrom,
		}
	}

	return routingTable, nil
}

// UpdateRoutingTable updates the routing table with received entries.
//   - #1 An already existing entry is updated if the new hop count is lower than the existing one.
//   - #2 New entries are added to the routing table.
//   - #3 Existing entries that are not present in the new entries are removed if the existing entry's next hop is the host that sent the update.
//   - #4 Entries that point to the local host are ignored.
//   - #5 If the received peer's IPv4 address is not in the routing table, it is added with a hop count of 1 and the next hop set to the receivedFrom address.
//
// For changed entries, the hop count is incremented by one and the NextHop is set to receivedFrom.
// Gets the routing table from another host and their IPv4 address as parameters.
// Returns true if the routing table was updated.
func UpdateRoutingTable(receivedTable RoutingTable, receivedFrom netip.AddrPort) bool {
	updated := false

	for hostIP, newEntry := range receivedTable.Entries {
		incrementedHopCount := newEntry.HopCount + 1
		existingEntry, exists := routingTable.Entries[hostIP]

		if !exists || incrementedHopCount < existingEntry.HopCount { // #1, #2
			if socket.GetLocalAddress().AddrPort().Addr() == hostIP {
				continue // #4
			}

			routingTable.Entries[hostIP] = RouteEntry{
				HopCount: incrementedHopCount,
				NextHop:  receivedFrom,
			}
			updated = true
		}
	}

	for hostIP, entry := range routingTable.Entries {
		_, existsInNewTable := receivedTable.Entries[hostIP]

		if !existsInNewTable && entry.NextHop == receivedFrom { // #3
			delete(routingTable.Entries, hostIP)
			updated = true
		}
	}

	routingTable.Entries[receivedFrom.Addr()] = RouteEntry{
		HopCount: 1,
		NextHop:  receivedFrom,
	} // #5; e.g. when receiving the first routing table update from a new peer

	return updated
}

// AddRoutingEntry adds a new entry to the routing table or updates an existing one if the new hop count is lower than the existing one.
func AddRoutingEntry(destinationIP netip.Addr, hopCount int, nextHop netip.AddrPort) {
	existingEntry, exists := routingTable.Entries[destinationIP]
	if exists && hopCount >= existingEntry.HopCount {
		return // Do not update if the new hop count is not lower
	}

	routingTable.Entries[destinationIP] = RouteEntry{
		HopCount: hopCount,
		NextHop:  nextHop,
	}
}

// RemoveRoutingEntry removes an entry from the routing table by its destination IP address.
// If the entry does not exist, it does nothing.
func RemoveRoutingEntry(destinationIP netip.Addr) {
	_, exists := routingTable.Entries[destinationIP]
	if !exists {
		return // No entry to remove
	}

	delete(routingTable.Entries, destinationIP)
}

// getNextHop returns the next hop for a given destination IP address.
func getNextHop(destinationIP netip.Addr) (addrPort netip.AddrPort, found bool) {
	entry, exists := routingTable.Entries[destinationIP]
	if !exists {
		return netip.AddrPort{}, false
	}

	return entry.NextHop, true
}

// GetRoutingTable returns the current routing table.
func GetRoutingTable() RoutingTable {
	return routingTable
}

// IsNeighbor checks if a given peer is a neighbor, meaning it is directly reachable with a hop count of 1.
func IsNeighbor(peer netip.Addr) bool {
	entry, exists := routingTable.Entries[peer]
	if !exists {
		return false
	}

	return entry.HopCount == 1
}
