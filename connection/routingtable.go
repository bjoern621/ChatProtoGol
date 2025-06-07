package connection

import (
	"net/netip"

	"bjoernblessin.de/chatprotogol/socket"
)

type RouteEntry struct {
	HopCount int
	NextHop  netip.AddrPort // The IPv4 address and port of the next hop host
}

type RoutingTable struct {
	Entries map[netip.AddrPort]RouteEntry // Maps IPv4 addresses to route entries
}

var routingTable = RoutingTable{
	Entries: make(map[netip.AddrPort]RouteEntry),
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
		ipv4Bytes := destinationIP.Addr().As4()
		payload = append(payload, ipv4Bytes[:]...)
		payload = append(payload, byte(entry.HopCount))
	}

	return payload
}

// Update updates the routing table with received entries.
//   - #1 An already existing entry is updated if the new hop count is lower than the existing one.
//   - #2 New entries are added to the routing table.
//   - #3 Existing entries that are not present in the new entries are removed if the existing entry's next hop is the host that sent the update.
//   - #4 Entries that point to the local host are ignored.
//
// For changed entries, the hop count is incremented by one and the NextHop is set to receivedFrom.
// Gets the routing table from another host and their IPv4 address as parameters.
// Returns true if the routing table was updated.
func Update(receivedTable RoutingTable, receivedFrom netip.AddrPort) bool {
	updated := false

	for hostIP, newEntry := range receivedTable.Entries {
		incrementedHopCount := newEntry.HopCount + 1
		existingEntry, exists := routingTable.Entries[hostIP]

		if !exists || incrementedHopCount < existingEntry.HopCount { // #1, #2
			if socket.GetLocalAddress().AddrPort() == hostIP {
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

	return updated
}

// AddRoutingEntry adds a new entry to the routing table or updates an existing one if the new hop count is lower than the existing one.
func AddRoutingEntry(destinationIP netip.AddrPort, hopCount int, nextHop netip.AddrPort) {
	existingEntry, exists := routingTable.Entries[destinationIP]
	if exists && hopCount >= existingEntry.HopCount {
		return // Do not add or update if the new hop count is not lower
	}

	routingTable.Entries[destinationIP] = RouteEntry{
		HopCount: hopCount,
		NextHop:  nextHop,
	}
}

// getNextHop returns the next hop for a given destination IP address.
func getNextHop(destinationIP netip.AddrPort) (netip.AddrPort, bool) {
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
