package main

import (
	"testing"
)

func TestParseBulkLine(t *testing.T) {
	tests := []struct {
		input     string
		wantName  string
		wantCount int
		firstIP   string
		lastIP    string
	}{
		{
			input:     "10.0.0.2 - 10.0.0.150",
			wantName:  "",
			wantCount: 149,
			firstIP:   "10.0.0.2",
			lastIP:    "10.0.0.150",
		},
		{
			input:     "10.0.0.2-150",
			wantName:  "",
			wantCount: 149,
			firstIP:   "10.0.0.2",
			lastIP:    "10.0.0.150",
		},
		{
			input:     "10.0.0.2 - 150",
			wantName:  "",
			wantCount: 149,
			firstIP:   "10.0.0.2",
			lastIP:    "10.0.0.150",
		},
		{
			input:     "10.0.0.2 to 10.0.0.10",
			wantName:  "",
			wantCount: 9,
			firstIP:   "10.0.0.2",
			lastIP:    "10.0.0.10",
		},
		{
			input:     "10.0.0.0/29", // 8 IPs: .1 to .6 usable
			wantName:  "",
			wantCount: 6,
			firstIP:   "10.0.0.1",
			lastIP:    "10.0.0.6",
		},
		{
			input:     "10.0.0.50",
			wantName:  "",
			wantCount: 1,
			firstIP:   "10.0.0.50",
			lastIP:    "10.0.0.50",
		},
		{
			input:     "core-switch, 10.0.0.1",
			wantName:  "core-switch",
			wantCount: 1,
			firstIP:   "10.0.0.1",
			lastIP:    "10.0.0.1",
		},
		{
			input:     "10.0.0.1 core-switch",
			wantName:  "core-switch",
			wantCount: 1,
			firstIP:   "10.0.0.1",
			lastIP:    "10.0.0.1",
		},
	}

	for _, tc := range tests {
		name, hosts := parseBulkLine(tc.input)
		if name != tc.wantName {
			t.Errorf("input %q: got name %q, want %q", tc.input, name, tc.wantName)
		}
		if len(hosts) != tc.wantCount {
			t.Errorf("input %q: got %d hosts, want %d", tc.input, len(hosts), tc.wantCount)
		}
		if len(hosts) > 0 {
			if hosts[0] != tc.firstIP {
				t.Errorf("input %q: first IP got %s, want %s", tc.input, hosts[0], tc.firstIP)
			}
			if hosts[len(hosts)-1] != tc.lastIP {
				t.Errorf("input %q: last IP got %s, want %s", tc.input, hosts[len(hosts)-1], tc.lastIP)
			}
		}
	}
}
