package netbind

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// NICInfo describes an active IPv4 network interface on this computer.
type NICInfo struct {
	Name      string `json:"name"`
	IP        string `json:"ip"`
	CIDR      string `json:"cidr"`
	MaskBits  int    `json:"maskBits"`
	RangeHint string `json:"rangeHint"`
	MAC       string `json:"mac"`
	Kind      string `json:"kind"` // "ethernet", "wifi", "virtual"
}

// DiscoveredHost is a live device found by ScanSubnet on port 22, 443, or 80.
type DiscoveredHost struct {
	IP           string `json:"ip"`
	Ports        []int  `json:"ports"`
	FamilyGuess  string `json:"familyGuess"` // "aos-cx", "aos-s", "instant-on", ""
	WebOnly      bool   `json:"webOnly"`
	SSHBanner    string `json:"sshBanner,omitempty"`
	ExistingID   string `json:"existingId,omitempty"`
	ExistingName string `json:"existingName,omitempty"`
}

var (
	mu      sync.RWMutex
	boundIP string
	bound   net.IP
)

// SetBindIP sets the local IPv4 address used as the source interface for
// outbound switch connections and scans. Empty string uses the OS default route.
func SetBindIP(ipStr string) {
	mu.Lock()
	defer mu.Unlock()
	ipStr = strings.TrimSpace(ipStr)
	ip := net.ParseIP(ipStr)
	if ip != nil {
		ip = ip.To4()
	}
	if ip == nil {
		boundIP = ""
		bound = nil
		return
	}
	boundIP = ip.String()
	bound = ip
}

// GetBindIP returns the currently bound local IPv4 address, or "" for Auto.
func GetBindIP() string {
	mu.RLock()
	defer mu.RUnlock()
	return boundIP
}

func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// DialContext connects to addr over TCP, binding to the selected local NIC IP
// when one is active (with automatic fallback if the OS routing table requires
// another interface for a different subnet).
func DialContext(ctx context.Context, network, addr string, timeout time.Duration) (net.Conn, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	mu.RLock()
	localIP := bound
	mu.RUnlock()

	if localIP != nil && !isLoopbackAddr(addr) {
		d := net.Dialer{
			Timeout:   timeout,
			LocalAddr: &net.TCPAddr{IP: localIP},
		}
		conn, err := d.DialContext(ctx, network, addr)
		if err == nil {
			return conn, nil
		}
		// If the user explicitly bound a NIC, only fall back to unbound dial if
		// the OS rejected the local bind address (e.g., reaching a different routed subnet).
		if strings.Contains(strings.ToLower(err.Error()), "bind") ||
			strings.Contains(strings.ToLower(err.Error()), "unreachable") ||
			strings.Contains(strings.ToLower(err.Error()), "can't assign") ||
			strings.Contains(strings.ToLower(err.Error()), "invalid argument") {
			d2 := net.Dialer{Timeout: timeout}
			if c2, err2 := d2.DialContext(ctx, network, addr); err2 == nil {
				return c2, nil
			}
		}
		return nil, err
	}

	d := net.Dialer{Timeout: timeout}
	return d.DialContext(ctx, network, addr)
}

// DialTimeout is a convenience wrapper around DialContext.
func DialTimeout(network, addr string, timeout time.Duration) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return DialContext(ctx, network, addr, timeout)
}

func classifyNIC(name string) string {
	l := strings.ToLower(name)
	for _, k := range []string{"vethernet", "vmware", "vbox", "virtual", "wsl", "hyper-v", "docker", "tailscale", "zerotier", "wireguard", "utun", "tun", "tap", "loopback", "bridge", "npcap"} {
		if strings.Contains(l, k) {
			return "virtual"
		}
	}
	for _, k := range []string{"wi-fi", "wifi", "wlan", "wireless", "airport"} {
		if strings.Contains(l, k) {
			return "wifi"
		}
	}
	return "ethernet"
}

func kindRank(k string) int {
	switch k {
	case "ethernet":
		return 0
	case "wifi":
		return 1
	default:
		return 2
	}
}

// ListNICs returns all active non-loopback IPv4 interfaces on this computer.
func ListNICs() []NICInfo {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []NICInfo
	for _, iface := range ifs {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		kind := classifyNIC(iface.Name)
		mac := iface.HardwareAddr.String()
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok || ipNet.IP == nil {
				continue
			}
			ip4 := ipNet.IP.To4()
			if ip4 == nil || ip4.IsLoopback() || ip4.Equal(net.IPv4zero) {
				continue
			}
			ones, bits := ipNet.Mask.Size()
			if bits != 32 || ones <= 0 {
				ones = 24
			}
			netIP := ip4.Mask(ipNet.Mask)
			cidr := fmt.Sprintf("%s/%d", netIP.String(), ones)

			// For scan convenience, cap range hint to the local /24 when the mask is wider (e.g. /16)
			var rangeHint string
			if ones >= 24 && ones <= 30 {
				start, end := subnetHostBounds(ip4, ipNet.Mask)
				rangeHint = fmt.Sprintf("%s - %s", start, end)
			} else {
				rangeHint = fmt.Sprintf("%d.%d.%d.1 - %d.%d.%d.254", ip4[0], ip4[1], ip4[2], ip4[0], ip4[1], ip4[2])
			}
			out = append(out, NICInfo{
				Name:      iface.Name,
				IP:        ip4.String(),
				CIDR:      cidr,
				MaskBits:  ones,
				RangeHint: rangeHint,
				MAC:       mac,
				Kind:      kind,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ri, rj := kindRank(out[i].Kind), kindRank(out[j].Kind)
		if ri != rj {
			return ri < rj
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].IP < out[j].IP
	})
	return out
}

func subnetHostBounds(ip4 net.IP, mask net.IPMask) (string, string) {
	ipInt := binary.BigEndian.Uint32(ip4)
	maskInt := binary.BigEndian.Uint32(mask)
	netInt := ipInt & maskInt
	bcastInt := netInt | ^maskInt
	first := netInt + 1
	last := bcastInt - 1
	if last < first {
		first, last = netInt, bcastInt
	}
	return uint32ToIP(first), uint32ToIP(last)
}

func uint32ToIP(v uint32) string {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	return fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3])
}

// ExpandScanTarget turns a CIDR ("10.0.0.0/24"), range ("10.0.0.1 - 10.0.0.254" or "10.0.0.1-50"),
// or single IP into a list of IPv4 addresses (capped at 512 hosts for fast discovery).
func ExpandScanTarget(target string) ([]string, error) {
	t := strings.TrimSpace(target)
	if t == "" {
		return nil, fmt.Errorf("masukkan subnet CIDR atau range IP untuk di-scan")
	}
	const maxHosts = 512
	if strings.Contains(t, "/") {
		ip, ipNet, err := net.ParseCIDR(t)
		if err != nil || ip.To4() == nil {
			return nil, fmt.Errorf("format CIDR tidak valid: %s", t)
		}
		ones, _ := ipNet.Mask.Size()
		// If someone passes a /16 or /8 from a wide NIC mask, scan the /24 around the IP
		if ones < 23 {
			ip4 := ip.To4()
			var out []string
			for i := 1; i <= 254; i++ {
				out = append(out, fmt.Sprintf("%d.%d.%d.%d", ip4[0], ip4[1], ip4[2], i))
			}
			return out, nil
		}
		maskInt := binary.BigEndian.Uint32(ipNet.Mask)
		netInt := binary.BigEndian.Uint32(ipNet.IP.To4()) & maskInt
		total := ^maskInt + 1
		first, last := netInt+1, netInt+total-2
		if total <= 2 {
			first, last = netInt, netInt+total-1
		}
		var out []string
		for cur := first; cur <= last && len(out) < maxHosts; cur++ {
			out = append(out, uint32ToIP(cur))
		}
		return out, nil
	}

	var sep string
	for _, s := range []string{" to ", "..", "-"} {
		if strings.Contains(t, s) {
			sep = s
			break
		}
	}
	if sep != "" {
		parts := strings.SplitN(t, sep, 2)
		fromStr, toStr := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		ipFrom := net.ParseIP(fromStr).To4()
		if ipFrom == nil {
			return nil, fmt.Errorf("IP awal tidak valid: %s", fromStr)
		}
		fromInt := binary.BigEndian.Uint32(ipFrom)
		var toInt uint32
		if n, err := strconv.Atoi(toStr); err == nil && n >= 1 && n <= 254 {
			endIP := make(net.IP, 4)
			copy(endIP, ipFrom)
			endIP[3] = byte(n)
			toInt = binary.BigEndian.Uint32(endIP)
		} else if ipTo := net.ParseIP(toStr).To4(); ipTo != nil {
			toInt = binary.BigEndian.Uint32(ipTo)
		} else {
			return nil, fmt.Errorf("IP akhir tidak valid: %s", toStr)
		}
		if toInt < fromInt {
			fromInt, toInt = toInt, fromInt
		}
		var out []string
		for cur := fromInt; cur <= toInt && len(out) < maxHosts; cur++ {
			out = append(out, uint32ToIP(cur))
		}
		return out, nil
	}

	if ip := net.ParseIP(t).To4(); ip != nil {
		// If a single IP is given, scan that IP
		return []string{ip.String()}, nil
	}
	return nil, fmt.Errorf("format IP / subnet tidak dikenali: %s", t)
}

// ScanSubnet probes candidate IPv4 hosts in parallel on ports 22 (SSH), 443 (HTTPS),
// and 80 (HTTP) using the active bound NIC, returning responsive devices sorted by IP.
func ScanSubnet(ctx context.Context, target string, existingByIP map[string][2]string) ([]DiscoveredHost, error) {
	ips, err := ExpandScanTarget(target)
	if err != nil {
		return nil, err
	}
	localIPs := map[string]bool{}
	for _, nic := range ListNICs() {
		localIPs[nic.IP] = true
	}

	type job struct {
		idx int
		ip  string
	}
	jobs := make(chan job, len(ips))
	for i, ip := range ips {
		if localIPs[ip] {
			continue
		}
		jobs <- job{idx: i, ip: ip}
	}
	close(jobs)

	var (
		resMu sync.Mutex
		found []DiscoveredHost
		wg    sync.WaitGroup
	)
	workers := 64
	if len(ips) < workers {
		workers = len(ips)
	}
	if workers < 1 {
		workers = 1
	}

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				if ctx.Err() != nil {
					return
				}
				if h, ok := probeCandidateHost(ctx, j.ip, existingByIP); ok {
					resMu.Lock()
					found = append(found, h)
					resMu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	sort.Slice(found, func(i, j int) bool {
		ai := net.ParseIP(found[i].IP).To4()
		aj := net.ParseIP(found[j].IP).To4()
		if ai != nil && aj != nil {
			return binary.BigEndian.Uint32(ai) < binary.BigEndian.Uint32(aj)
		}
		return found[i].IP < found[j].IP
	})
	return found, nil
}

func probeCandidateHost(ctx context.Context, ip string, existingByIP map[string][2]string) (DiscoveredHost, bool) {
	const timeout = 550 * time.Millisecond
	var openPorts []int
	var banner string

	// 1. Check SSH port 22 first (and grab SSH identification banner if available)
	if conn, err := DialContext(ctx, "tcp", net.JoinHostPort(ip, "22"), timeout); err == nil {
		openPorts = append(openPorts, 22)
		_ = conn.SetReadDeadline(time.Now().Add(350 * time.Millisecond))
		rd := bufio.NewReader(conn)
		if line, err := rd.ReadString('\n'); err == nil {
			banner = strings.TrimSpace(line)
		}
		_ = conn.Close()
	}

	// 2. Check HTTPS 443 and HTTP 80
	for _, p := range []int{443, 80} {
		if conn, err := DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(p)), 400*time.Millisecond); err == nil {
			openPorts = append(openPorts, p)
			_ = conn.Close()
		}
	}

	if len(openPorts) == 0 {
		return DiscoveredHost{}, false
	}

	h := DiscoveredHost{
		IP:        ip,
		Ports:     openPorts,
		SSHBanner: banner,
	}
	hasSSH := false
	for _, p := range openPorts {
		if p == 22 {
			hasSSH = true
		}
	}
	bl := strings.ToLower(banner)
	switch {
	case strings.Contains(bl, "mocana"):
		h.FamilyGuess = "aos-s"
	case strings.Contains(bl, "openssh"):
		h.FamilyGuess = "aos-cx"
	case !hasSSH:
		h.WebOnly = true
		h.FamilyGuess = "instant-on"
	}
	if ex, ok := existingByIP[ip]; ok {
		h.ExistingID = ex[0]
		h.ExistingName = ex[1]
	}
	return h, true
}
