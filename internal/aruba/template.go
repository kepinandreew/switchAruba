package aruba

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// TemplateVLAN stores one VLAN definition inside a reusable switch template
// (explicitly excluding management IP addresses so applying never causes an IP conflict).
type TemplateVLAN struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	IGMPSnooping bool   `json:"igmpSnooping,omitempty"`
}

// TemplatePort stores reusable per-port settings (description, VLANs, QoS/EEE).
type TemplatePort struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Mode        string `json:"mode,omitempty"` // "access" or "trunk"
	NativeVLAN  int    `json:"nativeVlan,omitempty"`
	TaggedVLANs []int  `json:"taggedVlans,omitempty"`
	QoSTrust    string `json:"qosTrust,omitempty"`
	EEEOff      bool   `json:"eeeOff,omitempty"`
}

// SwitchTemplate is a device-independent switch configuration template with
// all device-unique identity (IP addresses, default gateway, MACs, serials,
// passwords, SSH keys) stripped out.
type SwitchTemplate struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Notes        string         `json:"notes,omitempty"`
	SourceModel  string         `json:"sourceModel,omitempty"`
	SourceFamily Family         `json:"sourceFamily,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
	VLANs        []TemplateVLAN `json:"vlans"`
	Ports        []TemplatePort `json:"ports,omitempty"`
	DanteQoS     bool           `json:"danteQos,omitempty"`
	JumboMTU     bool           `json:"jumboMtu,omitempty"`
	STPEnabled   bool           `json:"stpEnabled,omitempty"`
	Timezone     string         `json:"timezone,omitempty"`
	NTPServers   []string       `json:"ntpServers,omitempty"`
}

// TemplateApplyRequest controls which sections of a SwitchTemplate are applied
// to a target switch.
type TemplateApplyRequest struct {
	TemplateID       string   `json:"templateId"`
	NewHostname      string   `json:"newHostname,omitempty"`
	IncludeVLANs     bool     `json:"includeVlans"`
	IncludeIGMP      bool     `json:"includeIgmp"`
	IGMPQuerier      bool     `json:"igmpQuerier"`
	IncludeDanteQoS  bool     `json:"includeDanteQos"`
	JumboMTU         bool     `json:"jumboMtu"`
	IncludePortVLANs bool     `json:"includePortVlans"`
	IncludePortNames bool     `json:"includePortNames"`
	IncludeSystem    bool     `json:"includeSystem"`
	SkipPorts        []string `json:"skipPorts,omitempty"`
	Save             bool     `json:"save"`
}

// ExtractTemplateFromSnapshot creates a clean, IP-free SwitchTemplate from a
// collected switch Snapshot.
func ExtractTemplateFromSnapshot(name, notes string, snap *Snapshot) SwitchTemplate {
	tpl := SwitchTemplate{
		Name:      strings.TrimSpace(name),
		Notes:     strings.TrimSpace(notes),
		CreatedAt: time.Now(),
	}
	if snap == nil {
		return tpl
	}
	tpl.SourceModel = snap.Facts.Model
	tpl.SourceFamily = snap.Family
	tpl.STPEnabled = snap.STP.Enabled
	tpl.Timezone = snap.Security.Timezone
	if len(snap.Security.NTPServers) > 0 {
		tpl.NTPServers = append([]string(nil), snap.Security.NTPServers...)
	}
	if snap.QoS != nil && (snap.QoS.GlobalTrust == "dscp" || snap.QoS.DSCP56Priority >= 6 || snap.QoS.DSCP46Priority >= 5) {
		tpl.DanteQoS = true
	}
	lowCfg := strings.ToLower(snap.RunningConfig)
	if strings.Contains(lowCfg, "mtu 9") || strings.Contains(lowCfg, "\n   jumbo") || strings.Contains(lowCfg, "\n  jumbo") {
		tpl.JumboMTU = true
	}

	untaggedByPort := map[string]int{}
	taggedByPort := map[string][]int{}
	for _, v := range snap.VLANs {
		if v.ID < 1 || v.ID > 4094 {
			continue
		}
		tpl.VLANs = append(tpl.VLANs, TemplateVLAN{
			ID:           v.ID,
			Name:         CleanPortName(v.Name),
			IGMPSnooping: v.IGMPSnooping,
		})
		for _, p := range v.Untagged {
			untaggedByPort[p] = v.ID
		}
		for _, p := range v.Tagged {
			taggedByPort[p] = append(taggedByPort[p], v.ID)
		}
	}
	sort.Slice(tpl.VLANs, func(i, j int) bool { return tpl.VLANs[i].ID < tpl.VLANs[j].ID })

	for _, p := range snap.Ports {
		if strings.HasPrefix(strings.ToLower(p.Name), "trk") || strings.HasPrefix(strings.ToLower(p.Name), "lag") {
			continue
		}
		native := untaggedByPort[p.Name]
		if native == 0 && p.VLAN != "" {
			if v, err := strconv.Atoi(strings.TrimSpace(p.VLAN)); err == nil && v > 0 {
				native = v
			}
		}
		tagged := dedupSortedInts(taggedByPort[p.Name])
		mode := "access"
		if len(tagged) > 0 || strings.EqualFold(p.Mode, "trunk") {
			mode = "trunk"
		}
		if native == 0 {
			native = 1
		}
		tpl.Ports = append(tpl.Ports, TemplatePort{
			Name:        p.Name,
			Description: CleanPortName(p.Description),
			Mode:        mode,
			NativeVLAN:  native,
			TaggedVLANs: tagged,
			QoSTrust:    p.QoSTrust,
			EEEOff:      p.EEEOff,
		})
	}
	return tpl
}

// ExtractTemplateFromConfigText parses a raw Aruba AOS-CX or AOS-S running-config
// text (stripping all IP addresses, hostnames, and credentials) into a SwitchTemplate.
func ExtractTemplateFromConfigText(name, notes, rawCfg string) (SwitchTemplate, error) {
	rawCfg = strings.TrimSpace(rawCfg)
	if rawCfg == "" {
		return SwitchTemplate{}, fmt.Errorf("isi konfigurasi tidak boleh kosong")
	}
	fam := FamilyAOSS
	low := strings.ToLower(rawCfg)
	if strings.Contains(low, "interface 1/1/") || strings.Contains(low, "ip igmp snooping") || strings.Contains(low, "vlan trunk allowed") || strings.Contains(low, "vlan access") {
		fam = FamilyAOSCX
	}
	snap := &Snapshot{
		Family:        fam,
		RunningConfig: rawCfg,
	}
	var stpOn bool
	if fam == FamilyAOSCX {
		snap.VLANs, snap.Security, stpOn = ParseConfigCX(rawCfg)
	} else {
		snap.VLANs, snap.Security, stpOn = ParseConfigAOSS(rawCfg)
	}
	snap.STP.Enabled = stpOn
	qos, eeeOff, _ := ParseQoSConfig(fam, rawCfg)
	snap.QoS = qos

	// Discover ports & descriptions from interface blocks and VLAN member lists
	portMap := map[string]*Port{}
	var portOrder []string
	getPort := func(pName string) *Port {
		if p, ok := portMap[pName]; ok {
			return p
		}
		p := &Port{Name: pName, Enabled: true, EEEOff: eeeOff[pName]}
		portMap[pName] = p
		portOrder = append(portOrder, pName)
		return p
	}
	for _, v := range snap.VLANs {
		for _, p := range v.Untagged {
			getPort(p)
		}
		for _, p := range v.Tagged {
			getPort(p)
		}
	}
	for _, b := range splitConfig(rawCfg) {
		h := strings.TrimSpace(b.header)
		if m := ifaceHeaderRe.FindStringSubmatch(h); m != nil && !strings.HasPrefix(m[1], "vlan") && m[1] != "loopback" && m[1] != "mgmt" {
			for _, pName := range ExpandPorts(m[1]) {
				if !safePort.MatchString(pName) {
					continue
				}
				p := getPort(pName)
				for _, l := range b.lines {
					switch {
					case strings.HasPrefix(l, "description "):
						p.Description = unquote(strings.TrimSpace(l[len("description "):]))
					case strings.HasPrefix(l, "name "):
						p.Description = unquote(strings.TrimSpace(l[len("name "):]))
					case l == "qos trust dscp":
						p.QoSTrust = "dscp"
					case l == "no energy-efficient-ethernet":
						p.EEEOff = true
					}
				}
			}
		}
	}
	for _, pName := range portOrder {
		snap.Ports = append(snap.Ports, *portMap[pName])
	}

	return ExtractTemplateFromSnapshot(name, notes, snap), nil
}

// BuildTemplateCommands compiles a SwitchTemplate + TemplateApplyRequest into
// Aruba AOS-CX or AOS-S CLI commands for a target switch, guaranteeing zero
// management IP or default-gateway collisions.
func BuildTemplateCommands(targetFamily Family, targetSnap *Snapshot, tpl SwitchTemplate, req TemplateApplyRequest) ([]string, error) {
	if targetFamily == FamilyInstantOn {
		return nil, fmt.Errorf("provisioning template via SSH tidak didukung pada switch Instant On")
	}
	if !req.IncludeVLANs && !req.IncludeDanteQoS && !req.JumboMTU && !req.IncludePortVLANs && !req.IncludePortNames && !req.IncludeSystem && strings.TrimSpace(req.NewHostname) == "" {
		return nil, fmt.Errorf("pilih minimal satu bagian template untuk diterapkan")
	}
	newHost := CleanPortName(strings.TrimSpace(req.NewHostname))

	skip := map[string]bool{}
	for _, p := range req.SkipPorts {
		skip[strings.TrimSpace(p)] = true
	}

	// Build target port lookup (supports cross-family 1/1/N <-> N mapping)
	targetPorts := map[string]string{} // normalized key -> actual target port name
	var allTargetPorts []string
	if targetSnap != nil && len(targetSnap.Ports) > 0 {
		for _, p := range targetSnap.Ports {
			if strings.HasPrefix(strings.ToLower(p.Name), "trk") || strings.HasPrefix(strings.ToLower(p.Name), "lag") {
				continue
			}
			allTargetPorts = append(allTargetPorts, p.Name)
			targetPorts[p.Name] = p.Name
			if strings.HasPrefix(p.Name, "1/1/") {
				short := strings.TrimPrefix(p.Name, "1/1/")
				if _, exists := targetPorts[short]; !exists {
					targetPorts[short] = p.Name
				}
			} else if _, err := strconv.Atoi(p.Name); err == nil {
				cxName := "1/1/" + p.Name
				if _, exists := targetPorts[cxName]; !exists {
					targetPorts[cxName] = p.Name
				}
			}
		}
	} else {
		for _, tp := range tpl.Ports {
			pName := tp.Name
			if targetFamily == FamilyAOSCX && !strings.Contains(pName, "/") {
				pName = "1/1/" + pName
			} else if targetFamily == FamilyAOSS && strings.HasPrefix(pName, "1/1/") {
				pName = strings.TrimPrefix(pName, "1/1/")
			}
			allTargetPorts = append(allTargetPorts, pName)
			targetPorts[tp.Name] = pName
			targetPorts[pName] = pName
		}
	}

	cmds := []string{"configure terminal"}

	// 1. Optional New Hostname
	if newHost != "" {
		if targetFamily == FamilyAOSCX {
			h := strings.ReplaceAll(newHost, " ", "-")
			cmds = append(cmds, "hostname "+h)
		} else {
			cmds = append(cmds, fmt.Sprintf(`hostname "%s"`, newHost))
		}
	}

	// 2. Global System Settings (STP & NTP — never touches IP or gateway)
	if req.IncludeSystem {
		if tpl.STPEnabled {
			cmds = append(cmds, "spanning-tree")
		}
		if len(tpl.NTPServers) > 0 {
			if targetFamily == FamilyAOSCX {
				cmds = append(cmds, "ntp enable")
				for _, srv := range tpl.NTPServers {
					if safeHostToken(srv) {
						cmds = append(cmds, "ntp server "+srv)
					}
				}
			} else {
				cmds = append(cmds, "timesync ntp", "ntp unicast")
				for _, srv := range tpl.NTPServers {
					if safeHostToken(srv) {
						cmds = append(cmds, "ntp server "+srv)
					}
				}
			}
		}
	}

	// 3. VLANs, IGMP Snooping, and AOS-S Jumbo Frames
	vlans := tpl.VLANs
	if len(vlans) == 0 {
		vlans = []TemplateVLAN{{ID: 1, Name: "DEFAULT_VLAN"}}
	}
	if req.IncludeVLANs {
		for _, v := range vlans {
			if v.ID < 1 || v.ID > 4094 {
				continue
			}
			cmds = append(cmds, fmt.Sprintf("vlan %d", v.ID))
			if v.Name != "" && !(v.ID == 1 && strings.EqualFold(v.Name, "DEFAULT_VLAN")) {
				if targetFamily == FamilyAOSCX {
					cmds = append(cmds, "name "+ cleanCXVLANName(v.Name))
				} else {
					cmds = append(cmds, fmt.Sprintf(`name "%s"`, CleanPortName(v.Name)))
				}
			}
			if req.IncludeIGMP && v.IGMPSnooping {
				if targetFamily == FamilyAOSCX {
					cmds = append(cmds, "ip igmp snooping enable")
				} else {
					cmds = append(cmds, "ip igmp")
					if req.IGMPQuerier {
						cmds = append(cmds, "ip igmp querier")
					}
				}
			}
			if req.JumboMTU && targetFamily == FamilyAOSS {
				cmds = append(cmds, "jumbo")
			}
			cmds = append(cmds, "exit")
		}
		if targetFamily == FamilyAOSCX && req.IncludeIGMP && req.IGMPQuerier {
			for _, v := range vlans {
				if v.IGMPSnooping {
					cmds = append(cmds, fmt.Sprintf("interface vlan %d", v.ID), "ip igmp enable", "ip igmp querier", "exit")
				}
			}
		}
	} else if req.JumboMTU && targetFamily == FamilyAOSS {
		for _, v := range vlans {
			cmds = append(cmds, fmt.Sprintf("vlan %d", v.ID), "jumbo", "exit")
		}
	}

	// 4. Global Dante QoS Profile
	if req.IncludeDanteQoS {
		if targetFamily == FamilyAOSCX {
			cmds = append(cmds,
				"qos trust dscp",
				"qos dscp-map 56 local-priority 7 name Dante_PTP",
				"qos dscp-map 46 local-priority 6 name Dante_Audio",
				"qos schedule-profile dante_strict",
				"strict queue 0",
				"strict queue 1",
				"strict queue 2",
				"strict queue 3",
				"strict queue 4",
				"strict queue 5",
				"strict queue 6",
				"strict queue 7",
				"exit",
				"apply qos queue-profile factory-default schedule-profile dante_strict",
			)
		} else {
			cmds = append(cmds,
				"qos type-of-service diff-services",
				"qos dscp-map 111000 priority 7",
				"qos dscp-map 101110 priority 6",
			)
		}
	}

	// 5. Per-Port VLANs, Descriptions, QoS/EEE, and AOS-CX Jumbo MTU
	if req.IncludePortVLANs || req.IncludePortNames {
		usedTarget := map[string]bool{}
		for _, tp := range tpl.Ports {
			dstPort, ok := targetPorts[tp.Name]
			if !ok || skip[dstPort] || usedTarget[dstPort] || !safePort.MatchString(dstPort) {
				continue
			}
			usedTarget[dstPort] = true
			var sub []string
			if req.IncludePortNames && tp.Description != "" {
				if targetFamily == FamilyAOSCX {
					sub = append(sub, "description "+CleanPortName(tp.Description))
				} else {
					sub = append(sub, fmt.Sprintf(`name "%s"`, CleanPortName(tp.Description)))
				}
			}
			if req.IncludePortVLANs {
				native := tp.NativeVLAN
				if native < 1 || native > 4094 {
					native = 1
				}
				tagged := dedupSortedInts(tp.TaggedVLANs)
				if targetFamily == FamilyAOSCX {
					sub = append(sub, "no shutdown", "no routing")
					if len(tagged) > 0 {
						allowed := dedupSortedInts(append([]int{native}, tagged...))
						sub = append(sub,
							fmt.Sprintf("vlan trunk native %d", native),
							"vlan trunk allowed "+intsCSV(allowed),
						)
					} else {
						sub = append(sub, fmt.Sprintf("vlan access %d", native))
					}
				} else {
					sub = append(sub, fmt.Sprintf("untagged vlan %d", native))
					if len(tagged) > 0 {
						sub = append(sub, "tagged vlan "+intsCSV(tagged))
					}
				}
			}
			if req.IncludeDanteQoS || tp.QoSTrust == "dscp" {
				if targetFamily == FamilyAOSCX {
					sub = append(sub, "qos trust dscp")
				}
			}
			if req.IncludeDanteQoS || tp.EEEOff {
				sub = append(sub, "no energy-efficient-ethernet")
			}
			if req.JumboMTU && targetFamily == FamilyAOSCX {
				sub = append(sub, "mtu 9198")
			}
			if len(sub) > 0 {
				cmds = append(cmds, "interface "+dstPort)
				cmds = append(cmds, sub...)
				cmds = append(cmds, "exit")
			}
		}
	} else if req.IncludeDanteQoS || (req.JumboMTU && targetFamily == FamilyAOSCX) {
		var applyPorts []string
		for _, p := range allTargetPorts {
			if !skip[p] && safePort.MatchString(p) {
				applyPorts = append(applyPorts, p)
			}
		}
		if targetFamily == FamilyAOSCX {
			for _, p := range applyPorts {
				cmds = append(cmds, "interface "+p)
				if req.IncludeDanteQoS {
					cmds = append(cmds, "qos trust dscp", "no energy-efficient-ethernet")
				}
				if req.JumboMTU {
					cmds = append(cmds, "mtu 9198")
				}
				cmds = append(cmds, "exit")
			}
		} else if req.IncludeDanteQoS && len(applyPorts) > 0 {
			cmds = append(cmds, "interface "+CompressPorts(applyPorts), "no energy-efficient-ethernet", "exit")
		}
	}

	cmds = append(cmds, "end")
	if req.Save {
		cmds = append(cmds, "write memory")
	}
	return cmds, nil
}

func safeHostToken(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == ':') {
			return false
		}
	}
	return true
}

func cleanCXVLANName(s string) string {
	s = CleanPortName(s)
	if strings.Contains(s, " ") {
		return `"` + s + `"`
	}
	return s
}

func intsCSV(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = strconv.Itoa(x)
	}
	return strings.Join(parts, ",")
}
