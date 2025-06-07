package connection

import (
	"bytes"
	"net/netip"
	"testing"
)

func TestRoutingTableFormatForPayload(t *testing.T) {
	tests := []struct {
		name     string
		table    RoutingTable
		expected []byte
	}{
		{
			name: "Single entry",
			table: RoutingTable{
				Entries: map[netip.Addr]routeEntry{
					netip.MustParseAddr("10.0.0.1"): {HopCount: 1, NextHop: "10.0.0.1"},
				}},
			expected: []byte{
				10, 0, 0, 1,
				1,
			},
		},
		{
			name: "Multiple entries",
			table: RoutingTable{
				Entries: map[netip.Addr]routeEntry{
					netip.MustParseAddr("10.0.0.1"): {HopCount: 1, NextHop: "10.0.0.1"},
					netip.MustParseAddr("10.0.0.3"): {HopCount: 1, NextHop: "10.0.0.3"},
					netip.MustParseAddr("10.0.0.4"): {HopCount: 2, NextHop: "10.0.0.3"},
				}},
			expected: []byte{
				10, 0, 0, 1,
				1,
				10, 0, 0, 3,
				1,
				10, 0, 0, 4,
				2,
			},
		},
		{
			name: "Empty table",
			table: RoutingTable{
				Entries: map[netip.Addr]routeEntry{},
			},
			expected: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.table.FormatForPayload()
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
