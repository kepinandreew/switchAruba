package aruba

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"strings"
	"sync"
)

//go:embed oui.tsv.gz
var ouiGZ []byte

var (
	ouiOnce sync.Once
	ouiDB   map[string]string
)

func loadOUI() {
	ouiDB = make(map[string]string, 40000)
	if len(ouiGZ) == 0 {
		return
	}
	zr, err := gzip.NewReader(bytes.NewReader(ouiGZ))
	if err != nil {
		return
	}
	defer zr.Close()
	sc := bufio.NewScanner(zr)
	for sc.Scan() {
		line := sc.Text()
		tab := strings.IndexByte(line, '\t')
		if tab <= 0 {
			continue
		}
		ouiDB[line[:tab]] = line[tab+1:]
	}
}

// LookupVendor returns the hardware manufacturer for a MAC address using the
// embedded offline IEEE OUI registry (zero internet, zero switch SSH calls).
// Supports 36-bit (MA-S), 28-bit (MA-M), and 24-bit (MA-L) prefixes, plus
// locally-administered / randomized MAC detection.
func LookupVendor(mac string) string {
	norm := NormMAC(mac)
	if norm == "" {
		return ""
	}
	hex := strings.ToUpper(strings.ReplaceAll(norm, ":", ""))
	if len(hex) < 6 {
		return ""
	}
	ouiOnce.Do(loadOUI)
	if len(hex) >= 9 {
		if v, ok := ouiDB[hex[:9]]; ok {
			return v
		}
	}
	if len(hex) >= 7 {
		if v, ok := ouiDB[hex[:7]]; ok {
			return v
		}
	}
	if v, ok := ouiDB[hex[:6]]; ok {
		return v
	}
	// Bit 1 of the first octet indicates a locally-administered (often randomized) MAC.
	switch hex[1] {
	case '2', '6', 'A', 'E':
		return "Private / Randomized"
	}
	return ""
}
