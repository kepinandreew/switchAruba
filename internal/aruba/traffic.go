package aruba

import (
	"regexp"
	"strconv"
	"strings"
)

// Live traffic. AOS-S can print the current rate of every port; AOS-CX only
// prints byte counters, so its rate is the change between two readings.

// Counter is how many bytes a port has received and sent since it came up.
type Counter struct {
	Port    string `json:"port"`
	RxBytes uint64 `json:"rxBytes"`
	TxBytes uint64 `json:"txBytes"`
}

// Rate is the traffic on a port right now, in bits per second.
type Rate struct {
	Port  string  `json:"port"`
	RxBps float64 `json:"rxBps"`
	TxBps float64 `json:"txBps"`
	// Util is the percentage of the link speed the busier direction uses, or
	// -1 when the switch didn't say and the speed isn't known.
	Util float64 `json:"util"`
}

// TrafficCommand is the command that reads port traffic, and fallbacks.
func TrafficCommand(f Family) (string, []string) {
	switch f {
	case FamilyAOSCX:
		return "show interface statistics", []string{"show interface statistics non-zero"}
	default:
		return "show interfaces port-utilization", nil
	}
}

// RateIsDirect says the family's traffic command already prints rates.
func RateIsDirect(f Family) bool { return f != FamilyAOSCX }

var portTokenRe = regexp.MustCompile(`^(\d+/\d+/\d+|\d+/\d+|lag\d+|[A-Za-z]{0,3}\d+|Trk\d+)$`)

// splitColumns splits a table header on runs of two or more spaces.
func splitColumns(line string) []string {
	return regexp.MustCompile(`\s{2,}`).Split(strings.TrimSpace(line), -1)
}

func parseUint(s string) (uint64, bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	v, err := strconv.ParseUint(s, 10, 64)
	return v, err == nil
}

// ParseCXStatistics reads "show interface statistics". It understands the
// one-row-per-interface table (columns named "Rx Bytes" and "Tx Bytes"), and,
// as a fallback, the per-interface blocks of "show interface <name>".
func ParseCXStatistics(out string) []Counter {
	var rows []Counter
	var cols []string
	for _, line := range strings.Split(out, "\n") {
		t := strings.TrimSpace(line)
		low := strings.ToLower(t)
		if strings.HasPrefix(low, "interface") && strings.Contains(low, "bytes") {
			cols = nil
			for _, c := range splitColumns(t) {
				cols = append(cols, strings.ToLower(strings.Join(strings.Fields(c), " ")))
			}
			continue
		}
		if len(cols) == 0 {
			continue
		}
		f := strings.Fields(t)
		if len(f) < 3 || !portTokenRe.MatchString(f[0]) {
			continue
		}
		rxI, txI := -1, -1
		for i, c := range cols {
			switch c {
			case "rx bytes", "rx octets", "input bytes", "in bytes":
				rxI = i
			case "tx bytes", "tx octets", "output bytes", "out bytes":
				txI = i
			}
		}
		if rxI < 1 || txI < 1 || rxI >= len(f) || txI >= len(f) || len(f) != len(cols) {
			continue
		}
		rx, ok1 := parseUint(f[rxI])
		tx, ok2 := parseUint(f[txI])
		if ok1 && ok2 {
			rows = append(rows, Counter{Port: f[0], RxBytes: rx, TxBytes: tx})
		}
	}
	if len(rows) > 0 {
		return rows
	}
	return parseCXBlocks(out)
}

var (
	cxIfaceRe     = regexp.MustCompile(`(?i)^\s*(?:interface\s+)?(\d+/\d+/\d+|lag\d+)\b`)
	cxBytesRowRe  = regexp.MustCompile(`(?i)^\s*bytes\s+([\d,]+)\s+([\d,]+)(?:\s+[\d,]+)?\s*$`)
	cxRxBytesKVRe = regexp.MustCompile(`(?i)\brx[ _]?bytes\D+([\d,]+)`)
	cxTxBytesKVRe = regexp.MustCompile(`(?i)\btx[ _]?bytes\D+([\d,]+)`)
)

func parseCXBlocks(out string) []Counter {
	var rows []Counter
	cur := ""
	have := map[string]Counter{}
	var order []string
	set := func(f func(c *Counter)) {
		if cur == "" {
			return
		}
		c, ok := have[cur]
		if !ok {
			c = Counter{Port: cur}
			order = append(order, cur)
		}
		f(&c)
		have[cur] = c
	}
	for _, line := range strings.Split(out, "\n") {
		if m := cxIfaceRe.FindStringSubmatch(line); m != nil && !cxBytesRowRe.MatchString(line) {
			cur = m[1]
		}
		if m := cxBytesRowRe.FindStringSubmatch(line); m != nil {
			rx, ok1 := parseUint(m[1])
			tx, ok2 := parseUint(m[2])
			if ok1 && ok2 {
				set(func(c *Counter) { c.RxBytes, c.TxBytes = rx, tx })
			}
			continue
		}
		if m := cxRxBytesKVRe.FindStringSubmatch(line); m != nil {
			if v, ok := parseUint(m[1]); ok {
				set(func(c *Counter) { c.RxBytes = v })
			}
		}
		if m := cxTxBytesKVRe.FindStringSubmatch(line); m != nil {
			if v, ok := parseUint(m[1]); ok {
				set(func(c *Counter) { c.TxBytes = v })
			}
		}
	}
	for _, p := range order {
		rows = append(rows, have[p])
	}
	return rows
}

// rateNumber reads "520", "1,240", "1.5M" or "<1" as a plain number, scaled by
// the K/M/G suffix some firmware adds.
func rateNumber(s string) (float64, bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	s = strings.TrimPrefix(strings.TrimSuffix(s, "%"), "<")
	mul := 1.0
	if n := len(s); n > 0 {
		switch s[n-1] {
		case 'K', 'k':
			mul, s = 1e3, s[:n-1]
		case 'M', 'm':
			mul, s = 1e6, s[:n-1]
		case 'G', 'g':
			mul, s = 1e9, s[:n-1]
		}
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v * mul, true
}

// ParseAOSSUtilization reads "show interfaces port-utilization":
//
//	1   1000FDx | 520  42  0.05 | 1240  85  0.12
func ParseAOSSUtilization(out string) []Rate {
	var rates []Rate
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Split(line, "|")
		if len(parts) != 3 {
			continue
		}
		left := strings.Fields(parts[0])
		if len(left) == 0 || !portTokenRe.MatchString(left[0]) {
			continue
		}
		rx, tx := strings.Fields(parts[1]), strings.Fields(parts[2])
		if len(rx) < 1 || len(tx) < 1 {
			continue
		}
		rkb, ok1 := rateNumber(rx[0])
		tkb, ok2 := rateNumber(tx[0])
		if !ok1 || !ok2 {
			continue
		}
		util := -1.0
		if len(rx) >= 3 && len(tx) >= 3 {
			ru, a := rateNumber(rx[2])
			tu, b := rateNumber(tx[2])
			if a && b {
				util = ru
				if tu > util {
					util = tu
				}
			}
		}
		rates = append(rates, Rate{Port: left[0], RxBps: rkb * 1000, TxBps: tkb * 1000, Util: util})
	}
	return rates
}

// RatesFromCounters turns two readings dt seconds apart into rates. A port
// whose counter went down (it was cleared, or wrapped) is skipped this time.
func RatesFromCounters(prev, cur []Counter, dt float64) []Rate {
	if dt <= 0 {
		return nil
	}
	old := make(map[string]Counter, len(prev))
	for _, c := range prev {
		old[c.Port] = c
	}
	var out []Rate
	for _, c := range cur {
		p, ok := old[c.Port]
		if !ok || c.RxBytes < p.RxBytes || c.TxBytes < p.TxBytes {
			continue
		}
		out = append(out, Rate{
			Port:  c.Port,
			RxBps: float64(c.RxBytes-p.RxBytes) * 8 / dt,
			TxBps: float64(c.TxBytes-p.TxBytes) * 8 / dt,
			Util:  -1,
		})
	}
	return out
}

// SpeedMbps reads a port speed such as "1000", "100", "10G" or "10GigFD".
func SpeedMbps(s string) int {
	s = strings.ToLower(strings.TrimSpace(s))
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
		i++
	}
	if i == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0
	}
	if i < len(s) && s[i] == 'g' {
		v *= 1000
	}
	return int(v)
}
