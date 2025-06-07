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
				Entries: map[netip.AddrPort]RouteEntry{
					netip.MustParseAddrPort("10.0.0.1:1234"): {HopCount: 1, NextHop: netip.MustParseAddrPort("10.0.0.1:1234")},
				}},
			expected: []byte{
				10, 0, 0, 1,
				1,
			},
		},
		{
			name: "Multiple entries",
			table: RoutingTable{
				Entries: map[netip.AddrPort]RouteEntry{
					netip.MustParseAddrPort("10.0.0.1:1234"): {HopCount: 1, NextHop: netip.MustParseAddrPort("10.0.0.3:1234")},
					netip.MustParseAddrPort("10.0.0.3:1234"): {HopCount: 1, NextHop: netip.MustParseAddrPort("10.0.0.3:1234")},
					netip.MustParseAddrPort("10.0.0.4:1234"): {HopCount: 2, NextHop: netip.MustParseAddrPort("10.0.0.3:1234")},
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
				Entries: map[netip.AddrPort]RouteEntry{},
			},
			expected: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routingTable = tt.table

			result := FormatRoutingTableForPayload()
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
