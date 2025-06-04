// Package routing manages directly connected hosts. It provides a way to deliver messages to these hosts.
package routing

type routeEntry struct {
	HopCount int
	NextHop  string // The IPv4 address of the next hop router
}

type RoutingTable struct {
	Entries map[string]routeEntry // Maps IPv4 addresses to route entries
}

func newRoutingTable() RoutingTable {
	return RoutingTable{
		Entries: make(map[string]routeEntry),
	}
}

// FormatForPayload formats the routing table for inclusion in a Routing Table Update Message.
// Format:
//
//  +--------+--------+--------+--------+--------+
//  |                                   |        |
//  |   IPv4 Address of Destination     |  Hop   |
//  |          (32 bits)                | Count  |
//  |                                   |(8 bits)|
//  +--------+--------+--------+--------+--------+
//  |                                   |        |
//  |   IPv4 Address of Destination     |  Hop   |
//  |          (32 bits)                | Count  |
//  |                                   |(8 bits)|
//  +--------+--------+--------+--------+--------+
//  |                                            |
//  |                 ...                        |
//  |                                            |
//  +--------+--------+--------+--------+--------+
//
func (rt RoutingTable) FormatForPayload() []byte {
	return nil
}

// Update updates the routing table with received entries.
// #1 An already existing entry is updated if the new hop count is lower than the existing one.
// #2 New entries are added to the routing table.
// #3 Existing entries that are not present in the new entries are removed if the existing entry's next hop is the host that sent the update.
// For changed entries, the hop count is incremented by one and the NextHop is set to receivedFrom.
// Gets the routing table from another host and their IPv4 address as parameters.
// Returns true if the routing table was updated.
func (rt RoutingTable) Update(receivedTable RoutingTable, receivedFrom string) bool {
	updated := false

	for hostIP, newEntry := range receivedTable.Entries {
		incrementedHopCount := newEntry.HopCount + 1
		existingEntry, exists := rt.Entries[hostIP]

		if !exists || incrementedHopCount < existingEntry.HopCount { // #1, #2
			rt.Entries[hostIP] = routeEntry{
				HopCount: incrementedHopCount,
				NextHop:  receivedFrom,
			}
			updated = true
		}
	}

	for hostIP, entry := range rt.Entries {
		_, existsInNewTable := receivedTable.Entries[hostIP]

		if !existsInNewTable && entry.NextHop == receivedFrom { // #3
			delete(rt.Entries, hostIP)
			updated = true
		}
	}

	return updated
}
