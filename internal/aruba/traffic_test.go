package aruba

import "testing"

func TestParseAOSSUtilization(t *testing.T) {
	out := ` Status and Counters - Port Utilization

                                 Rx                           Tx
  Port   Mode       | ------------------------- + -------------------------
                    | Kbits/sec   Pkts/sec  Util | Kbits/sec   Pkts/sec  Util
  ------ ---------- + ---------- --------- ----- + ---------- --------- -----
  1      1000FDx    | 520         42        0.05 | 1240        85        0.12
  2                 | 0           0         0    | 0           0         0
  3      100FDx     | 8,400       1250      8.40 | 45000       6200      45.00
  A1     10GigFD    | 1.5M        900       0.15 | 2M          1000      0.20
`
	got := ParseAOSSUtilization(out)
	if len(got) != 4 {
		t.Fatalf("got %d rows: %+v", len(got), got)
	}
	if got[0].Port != "1" || got[0].RxBps != 520000 || got[0].TxBps != 1240000 || got[0].Util != 0.12 {
		t.Fatalf("row 1: %+v", got[0])
	}
	if got[2].RxBps != 8400000 || got[2].Util != 45 {
		t.Fatalf("row 3: %+v", got[2])
	}
	if got[3].Port != "A1" || got[3].RxBps != 1.5e9 || got[3].TxBps != 2e9 {
		t.Fatalf("row A1: %+v", got[3])
	}
}

func TestParseCXStatisticsTable(t *testing.T) {
	out := `Interface   Rx Packets   Rx Bytes    Rx Errors   Tx Packets   Tx Bytes     Tx Errors
-----------------------------------------------------------------------------------
1/1/1       1000         125000      0           2000         250000       0
1/1/2       0            0           0           0            0            0
1/1/3       5,000        1,000,000   1           6,000        2,000,000    0
`
	got := ParseCXStatistics(out)
	if len(got) != 3 {
		t.Fatalf("got %+v", got)
	}
	if got[0].Port != "1/1/1" || got[0].RxBytes != 125000 || got[0].TxBytes != 250000 {
		t.Fatalf("row 1: %+v", got[0])
	}
	if got[2].RxBytes != 1000000 || got[2].TxBytes != 2000000 {
		t.Fatalf("row 3: %+v", got[2])
	}
}

func TestParseCXStatisticsBlocks(t *testing.T) {
	out := `Interface 1/1/1 is up
 Statistics            RX                TX                Total
 Packets                1000              2000              3000
 Bytes                  125000            250000            375000

Interface 1/1/2 is down
 Statistics            RX                TX                Total
 Bytes                  0                 0                 0
`
	got := ParseCXStatistics(out)
	if len(got) != 2 || got[0].Port != "1/1/1" || got[0].RxBytes != 125000 || got[0].TxBytes != 250000 {
		t.Fatalf("got %+v", got)
	}
}

func TestRatesFromCounters(t *testing.T) {
	prev := []Counter{{Port: "1/1/1", RxBytes: 1000, TxBytes: 2000}, {Port: "1/1/2", RxBytes: 500, TxBytes: 500}}
	cur := []Counter{{Port: "1/1/1", RxBytes: 1000 + 125000, TxBytes: 2000 + 250000}, {Port: "1/1/2", RxBytes: 10, TxBytes: 10}, {Port: "1/1/3", RxBytes: 5}}
	got := RatesFromCounters(prev, cur, 10)
	// 125000 bytes in 10 s = 100 000 bit/s; the cleared and the new port are skipped
	if len(got) != 1 || got[0].RxBps != 100000 || got[0].TxBps != 200000 {
		t.Fatalf("got %+v", got)
	}
}

func TestSpeedMbps(t *testing.T) {
	for in, want := range map[string]int{"1000": 1000, "100": 100, "10G": 10000, "10GigFD": 10000, "1G": 1000, "": 0, "auto": 0} {
		if got := SpeedMbps(in); got != want {
			t.Errorf("SpeedMbps(%q) = %d, want %d", in, got, want)
		}
	}
}
