package core

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"myswitch/internal/aruba"
	"myswitch/internal/store"
)

type Severity string

const (
	Critical Severity = "critical"
	Warning  Severity = "warning"
	Info     Severity = "info"
)

func (s Severity) rank() int {
	switch s {
	case Critical:
		return 0
	case Warning:
		return 1
	}
	return 2
}

type Issue struct {
	SwitchID string `json:"switchId"`
	// SwitchIDs lists the switches a network-wide issue is about, which has
	// no single SwitchID; the UI uses it to show the issue per site.
	SwitchIDs  []string `json:"switchIds,omitempty"`
	SwitchName string   `json:"switchName"`
	Severity   Severity `json:"severity"`
	Category   string   `json:"category"`
	Title      string   `json:"title"`
	Detail     string   `json:"detail"`
}

// evaluate turns one switch's status and last snapshot into issues.
func evaluate(sw store.Switch, st *Status, snap *aruba.Snapshot, set store.Settings) []Issue {
	name := displayName(sw, snap)
	var out []Issue
	add := func(sev Severity, cat, title, detail string) {
		out = append(out, Issue{SwitchID: sw.ID, SwitchName: name, Severity: sev, Category: cat, Title: title, Detail: detail})
	}

	if st.Checked && !st.Online && sw.Cloud != nil {
		d := st.PingError
		if !st.Since.IsZero() {
			d += " since " + st.Since.Format("Jan 2 15:04")
		}
		add(Critical, "Reachability", "Switch is offline", d)
	} else if st.Checked && !st.Online {
		d := "No answer on TCP " + fmt.Sprint(sw.Port)
		if !st.Since.IsZero() {
			d += " since " + st.Since.Format("Jan 2 15:04")
		}
		if st.PingError != "" {
			d += " (" + st.PingError + ")"
		}
		add(Critical, "Reachability", "Switch is offline", d)
	}
	if st.HostKeyChanged {
		if sw.Web {
			add(Critical, "Security", "Switch identity changed", "The switch presented a different HTTPS certificate than before, or stopped offering encrypted login. MySwitch stopped signing in to protect the password. If the switch was replaced or reset, trust it again from the switch page.")
		} else {
			add(Critical, "SSH", "SSH host key changed", "The switch presented a different host key than before. If the switch was replaced or reset, trust the new key from the switch page.")
		}
	}
	if st.AuthFailed && sw.Cloud != nil {
		add(Critical, "Instant On", "Instant On login failed", st.CollectError)
	} else if st.CollectError != "" && sw.Cloud != nil {
		add(Warning, "Instant On", "Couldn't read the Instant On portal", st.CollectError)
	} else if st.AuthFailed {
		add(Critical, "SSH", "SSH login failed", st.CollectError)
	} else if st.CollectError != "" && !st.HostKeyChanged && st.Online {
		add(Warning, "SSH", "Data collection failed", st.CollectError)
	}
	if snap == nil || snap.CollectedAt.IsZero() {
		return out
	}

	opticIssues(snap, add)
	for _, n := range snap.Notes {
		add(Severity(n.Severity), n.Category, n.Title, n.Detail)
	}
	if c := snap.Cloud; c != nil {
		for _, a := range c.Alerts {
			sev := Info
			switch a.Severity {
			case "major":
				sev = Critical
			case "minor":
				sev = Warning
			}
			d := "Reported by the Instant On portal"
			if !a.Raised.IsZero() {
				d += " since " + a.Raised.Format("Jan 2 15:04")
			}
			add(sev, "Instant On", a.Text, d)
		}
		if c.UpToDate != nil && !*c.UpToDate {
			add(Info, "Firmware", "Firmware update available", "Running "+orDash(snap.Facts.Software)+". Instant On updates are installed from the Instant On app or portal.")
		}
		if c.PoEBudgetW > 0 && c.PoEUsedW >= c.PoEBudgetW*0.9 {
			add(Warning, "Health", fmt.Sprintf("PoE budget nearly used: %.0f of %.0f W", c.PoEUsedW, c.PoEBudgetW), "New PoE devices may not get power.")
		}
	}
	var loops []string
	for _, p := range snap.Ports {
		if p.Loop {
			loops = append(loops, p.Name)
		}
	}
	if len(loops) > 0 {
		add(Critical, "Spanning tree", "Loop detected", "Ports: "+strings.Join(loops, ", ")+". Find the looped cable or device on these ports.")
	}

	// spanning tree
	if snap.STP.Known && !snap.STP.Enabled {
		add(Critical, "Spanning tree", "Spanning tree is disabled", "A single looped cable can take this segment down.")
	}
	if snap.STP.Enabled {
		if st.TopologyDelta > 0 {
			add(Warning, "Spanning tree", fmt.Sprintf("%d topology change(s) since last check", st.TopologyDelta), "Last change: "+orDash(snap.STP.LastChange)+". Frequent changes usually mean a flapping link or an edge port without admin-edge.")
		}
		if snap.STP.IsRoot && (snap.STP.BridgePriority == "32768" || snap.STP.BridgePriority == "8") {
			add(Warning, "Spanning tree", "Root bridge elected by default priority", "This switch is root only because of its MAC address. Set an explicit priority on the intended core.")
		}
		var blocking []string
		for _, p := range snap.STP.Ports {
			if p.State == "Blocking" || p.State == "Discarding" {
				blocking = append(blocking, p.Port)
			}
		}
		if len(blocking) > 0 {
			add(Info, "Spanning tree", fmt.Sprintf("%d port(s) blocking", len(blocking)), "Ports: "+strings.Join(blocking, ", ")+". Expected on redundant links.")
		}
	}

	// management plane security
	sec := snap.Security
	if sec.TelnetEnabled {
		fix := "Disable with: no telnet-server"
		if snap.Family == aruba.FamilyAOSCX {
			fix = "Disable with: no telnet-server vrf <vrf>"
		}
		seen := "The switch accepts connections on TCP port 23. "
		if sec.TelnetPort == "" {
			seen = ""
		}
		add(Critical, "Security", "Telnet is enabled", seen+"Credentials travel in clear text. "+fix)
	}
	if snap.Family == aruba.FamilyAOSS && sec.HTTPPlain {
		seen := "The switch answers on TCP port 80. "
		if sec.HTTPPort == "" {
			seen = ""
		}
		add(Warning, "Security", "Plain HTTP web management is enabled", seen+"Use web-management ssl and disable plaintext.")
	}
	if sec.SNMPDefault && !set.IgnoreSNMP {
		add(Critical, "Security", "Default SNMP community in use", "Community \"public\" or \"private\" is configured.")
	}
	if weak := aruba.WeakSSH(sec); len(weak) > 0 {
		fix := "On AOS-S, restrict them with ip ssh cipher and ip ssh mac."
		if snap.Family == aruba.FamilyAOSCX {
			fix = "On AOS-CX, restrict them with ssh ciphers, ssh macs and ssh key-exchange-algorithms."
		}
		add(Warning, "Security", "Outdated SSH algorithms enabled", "The SSH server still offers "+strings.Join(weak, ", ")+". "+fix)
	}
	if snap.RunningConfig != "" {
		// No time zone line means UTC. That's only worth a note when this
		// computer isn't on UTC/GMT itself (issue #24: right for London).
		if sec.Timezone == "" && snap.Family != aruba.FamilyInstantOn && !localIsUTC() {
			fix := "Set it with: time timezone <minutes from UTC>, or clock timezone on AOS-CX."
			add(Info, "Management", "No time zone set", "The switch's clock and log timestamps run on UTC. "+fix)
		}
		if len(sec.NTPServers) == 0 {
			add(Warning, "Management", "No NTP/SNTP server", "Log timestamps and certificates depend on correct time.")
		}
		if len(sec.LogServers) == 0 {
			add(Info, "Management", "No syslog server", "Events are only kept in the switch's local buffer.")
		}
		if len(sec.AAA) == 0 {
			add(Info, "Management", "Local accounts only", "No RADIUS/TACACS+ authentication configured.")
		}
		if snap.Family == aruba.FamilyAOSS && !sec.ManagerPassword {
			add(Critical, "Security", "No manager password set", "Anyone with console or network access gets full rights.")
		}
	}
	if pa := snap.PortAccess; pa != nil && len(pa.Ports) > 0 {
		var ports []string
		dot1x := false
		for _, a := range pa.Ports {
			ports = append(ports, a.Port)
			dot1x = dot1x || a.Dot1x
		}
		list := aruba.CompressPorts(ports)
		if len(pa.Servers) == 0 {
			add(Warning, "Port access", "Port authentication without a RADIUS server", "Ports "+list+" ask clients to log in, but no RADIUS server is set, so nobody can.")
		}
		if dot1x && !pa.Dot1xOn {
			add(Info, "Port access", "802.1X is set on ports but turned off", "The ports are configured for 802.1X, but it isn't turned on for the switch, so they don't ask for a login.")
		}
	}
	if snap.Facts.ConfigSaved != nil && !*snap.Facts.ConfigSaved {
		add(Warning, "Config", "Unsaved configuration changes", "Running config differs from startup. Changes will be lost on reboot.")
	}

	// hardware
	for _, e := range snap.Env {
		if !e.OK {
			kind := map[string]string{"fan": "Fan", "psu": "Power supply", "temp": "Temperature"}[e.Kind]
			add(Critical, "Hardware", kind+" problem", e.Status)
		}
	}
	if snap.Facts.CPU >= 0 && set.CPUWarn > 0 && snap.Facts.CPU >= set.CPUWarn {
		add(Warning, "Health", fmt.Sprintf("High CPU: %d%%", snap.Facts.CPU), "")
	}
	if snap.Facts.Memory >= 0 && set.MemWarn > 0 && snap.Facts.Memory >= set.MemWarn {
		add(Warning, "Health", fmt.Sprintf("High memory: %d%%", snap.Facts.Memory), "")
	}
	// PoE / switch power budget monitoring (zero extra SSH — from PSU status or Instant On cloud)
	if snap.Cloud != nil && snap.Cloud.PoEBudgetW > 0 {
		pct := int(math.Round(snap.Cloud.PoEUsedW / snap.Cloud.PoEBudgetW * 100))
		if pct >= 90 {
			add(Critical, "Power", fmt.Sprintf("PoE power budget critical (%d%%)", pct),
				fmt.Sprintf("%.1f W of %.0f W PoE budget in use. Powered devices may be denied power.", snap.Cloud.PoEUsedW, snap.Cloud.PoEBudgetW))
		} else if pct >= 80 {
			add(Warning, "Power", fmt.Sprintf("PoE power budget high (%d%%)", pct),
				fmt.Sprintf("%.1f W of %.0f W PoE budget in use.", snap.Cloud.PoEUsedW, snap.Cloud.PoEBudgetW))
		}
	} else if snap.Facts.PowerMaxW > 0 {
		pct := int(math.Round(snap.Facts.PowerUsedW / snap.Facts.PowerMaxW * 100))
		if pct >= 90 {
			add(Critical, "Power", fmt.Sprintf("Switch power / PoE load critical (%d%%)", pct),
				fmt.Sprintf("%.0f W of %.0f W PSU capacity in use. Check PoE draw or add redundant power.", snap.Facts.PowerUsedW, snap.Facts.PowerMaxW))
		} else if pct >= 80 {
			add(Warning, "Power", fmt.Sprintf("Switch power / PoE load high (%d%%)", pct),
				fmt.Sprintf("%.0f W of %.0f W PSU capacity in use.", snap.Facts.PowerUsedW, snap.Facts.PowerMaxW))
		}
	}

	// Port speed downgrade & half-duplex detection (zero extra SSH)
	infraPorts := map[string]bool{}
	for _, nb := range snap.Neighbors {
		c := strings.ToLower(nb.Capabilities)
		if strings.Contains(c, "bridge") || strings.Contains(c, "router") || strings.Contains(c, "wlan") || strings.Contains(c, "access") {
			infraPorts[nb.LocalPort] = true
		}
	}
	var halfDup, slowInfra, slowAccess []string
	for _, p := range snap.Ports {
		if !p.Enabled || !p.Up || strings.TrimSpace(p.Speed) == "" {
			continue
		}
		sl := strings.ToLower(p.Speed)
		pLbl := p.Name
		if p.Description != "" {
			pLbl += " (" + p.Description + ")"
		}
		if strings.Contains(sl, "hdx") || strings.Contains(sl, "half") {
			halfDup = append(halfDup, fmt.Sprintf("%s %s", pLbl, p.Speed))
		}
		mbps := aruba.SpeedMbps(p.Speed)
		if mbps > 0 && mbps <= 100 && isGigCapablePort(snap.Family, p.Type) {
			entry := fmt.Sprintf("%s %s", pLbl, p.Speed)
			if infraPorts[p.Name] || p.Uplink {
				slowInfra = append(slowInfra, entry)
			} else {
				slowAccess = append(slowAccess, entry)
			}
		}
	}
	if len(halfDup) > 0 {
		sort.Strings(halfDup)
		add(Warning, "Ports", fmt.Sprintf("%d port(s) running at half-duplex", len(halfDup)),
			"Half-duplex causes collisions and severe packet loss: "+strings.Join(halfDup, ", ")+". Check cable termination and speed/duplex auto-negotiation.")
	}
	if len(slowInfra) > 0 {
		sort.Strings(slowInfra)
		add(Warning, "Ports", fmt.Sprintf("%d uplink/AP port(s) downgraded to 100M/10M", len(slowInfra)),
			"Gigabit uplink or access-point links negotiated below 1 Gbps: "+strings.Join(slowInfra, ", ")+". Usually a damaged pair or loose RJ45 crimp.")
	}
	if len(slowAccess) > 0 {
		sort.Strings(slowAccess)
		add(Info, "Ports", fmt.Sprintf("%d Gigabit port(s) running at 100M/10M", len(slowAccess)),
			"Links below 1 Gbps (normal for 100M printers/controllers, or check cable if device supports 1G): "+strings.Join(slowAccess, ", ")+".")
	}

	// multicast / IGMP snooping on audio, DANTE, video or DSCP-trusted VLANs
	dscpPorts := map[string]bool{}
	for _, p := range snap.Ports {
		if p.QoSTrust == "dscp" {
			dscpPorts[p.Name] = true
		}
	}
	for _, v := range snap.VLANs {
		if !v.IGMPConfigKnown || (len(v.Untagged)+len(v.Tagged) == 0) || !isMulticastPriorityVLAN(v, dscpPorts) {
			continue
		}
		if !v.IGMPSnooping {
			fix := fmt.Sprintf("Enable in Edit VLAN → IGMP Snooping, or CLI: vlan %d → ip igmp", v.ID)
			if snap.Family == aruba.FamilyAOSCX {
				fix = fmt.Sprintf("Enable in Edit VLAN → IGMP Snooping, or CLI: vlan %d → ip igmp snooping enable", v.ID)
			}
			add(Warning, "Multicast", fmt.Sprintf("IGMP snooping disabled on %s", vlanLabel(v)),
				"Multicast traffic (such as DANTE/audio) can flood all ports and add switch CPU load. "+fix)
		}
	}

	// Dante / audio QoS readiness audit (zero extra SSH — parsed from running-config)
	if snap.QoS != nil && snap.QoS.Known {
		portByName := map[string]aruba.Port{}
		for _, p := range snap.Ports {
			portByName[p.Name] = p
		}
		var danteVLANs []aruba.VLAN
		dantePortSet := map[string]bool{}
		for _, v := range snap.VLANs {
			if (len(v.Untagged)+len(v.Tagged) == 0) || !isAudioDanteVLAN(v) {
				continue
			}
			danteVLANs = append(danteVLANs, v)
			for _, p := range v.Untagged {
				dantePortSet[p] = true
			}
			for _, p := range v.Tagged {
				dantePortSet[p] = true
			}
		}
		if len(danteVLANs) > 0 {
			globalDSCP := snap.QoS.GlobalTrust == "dscp"
			if snap.Family == aruba.FamilyAOSS {
				if !globalDSCP {
					add(Warning, "Dante QoS", fmt.Sprintf("DSCP QoS not enabled for %s", vlanLabel(danteVLANs[0])),
						"AOS-S switches need 'qos type-of-service diff-services' to honor Dante DSCP markings. Use Dante AV Profile to apply DSCP trust, queues, and IGMP.")
				}
			} else if snap.Family == aruba.FamilyAOSCX {
				var missingTrust []string
				var eeeEnabled []string
				for pName := range dantePortSet {
					p, ok := portByName[pName]
					if !ok {
						continue
					}
					trusted := p.QoSTrust == "dscp" || (globalDSCP && p.QoSTrust == "")
					if !trusted {
						missingTrust = append(missingTrust, pName)
					}
					if p.EEEOn {
						eeeEnabled = append(eeeEnabled, pName)
					}
				}
				if len(missingTrust) > 0 {
					sort.Strings(missingTrust)
					add(Warning, "Dante QoS", fmt.Sprintf("DSCP trust missing on %d audio/uplink port(s)", len(missingTrust)),
						"Ports carrying "+vlanLabel(danteVLANs[0])+" without DSCP trust: "+aruba.CompressPorts(missingTrust)+". Use Dante AV Profile to apply qos trust dscp.")
				}
				if len(eeeEnabled) > 0 {
					sort.Strings(eeeEnabled)
					add(Warning, "Dante QoS", fmt.Sprintf("Energy-Efficient Ethernet (EEE) enabled on %d audio port(s)", len(eeeEnabled)),
						"EEE causes micro-sleep latency and Dante clock dropouts on ports: "+aruba.CompressPorts(eeeEnabled)+". Disable with 'no energy-efficient-ethernet'.")
				}
				if !snap.QoS.StrictTopQueues {
					add(Info, "Dante QoS", "Strict priority QoS queue not applied for Dante",
						"AOS-CX defaults to equal-weight DWRR. Apply Dante AV Profile to map DSCP 56 (PTP) & 46 (Audio) to strict priority queues.")
				}
			}
		}
	}

	// logs
	major := 0
	var sample string
	for _, l := range snap.Logs {
		if l.Severity == "M" || l.Severity == "E" {
			major++
			if sample == "" {
				sample = l.Text
			}
		}
	}
	if major > 0 {
		add(Warning, "Logs", fmt.Sprintf("%d major/error event(s) in recent log", major), sample)
	}
	if n := aruba.FailedLogins(snap.Logs); n >= 3 {
		add(Warning, "Security", fmt.Sprintf("%d failed login attempts in the recent log", n), "Someone is trying passwords on this switch, or a script has a wrong one. The log tab has the details.")
	}
	if flaps := aruba.PortFlaps(snap.Logs); len(flaps) > 0 {
		var names []string
		for p, n := range flaps {
			if n >= 3 {
				names = append(names, fmt.Sprintf("%s (%d×)", p, n))
			}
		}
		if len(names) > 0 {
			sort.Strings(names)
			add(Warning, "Ports", fmt.Sprintf("%d port(s) flapping", len(names)), "Went down several times in the recent log: "+strings.Join(names, ", ")+". Usually a bad cable, transceiver or a device rebooting.")
		}
	}
	if len(st.UsersAdded) > 0 || len(st.UsersRemoved) > 0 {
		d := ""
		if len(st.UsersAdded) > 0 {
			d += "Added: " + strings.Join(st.UsersAdded, ", ") + ". "
		}
		if len(st.UsersRemoved) > 0 {
			d += "Removed: " + strings.Join(st.UsersRemoved, ", ") + ". "
		}
		add(Warning, "Security", "Local user accounts changed", d+"Seen "+st.UsersChanged.Format("Jan 2 15:04")+". If nobody on your team did this, treat it as a break-in.")
	}
	if stk := snap.Stack.Normalize(); stk != nil {
		for _, mb := range stk.Members {
			if !mb.OK {
				add(Critical, "Hardware", fmt.Sprintf("Stack member %s is %s", mb.ID, strings.ToLower(mb.Status)), strings.TrimSpace(mb.Model+" "+mb.MAC))
			}
		}
		if len(stk.Members) > 1 && strings.EqualFold(stk.Topology, "chain") {
			add(Warning, "Hardware", "Stack ring is broken", "The stack runs as a chain, so one more failed link splits it. Check the stacking cables.")
		}
	}
	if snap.CollectedAt.Before(time.Now().Add(-6 * time.Hour)) {
		add(Info, "SSH", "Data is more than 6 hours old", "Last collected "+snap.CollectedAt.Format("Jan 2 15:04"))
	}
	return out
}

// networkIssues looks across switches (e.g. more than one STP root). Roots
// are only compared between switches joined by layer 2 links on the map:
// branches connected through routers each have their own spanning tree, and
// a root per branch is expected.
func networkIssues(snaps map[string]*aruba.Snapshot, names map[string]string, topo Topology) []Issue {
	parent := map[string]string{}
	var find func(string) string
	find = func(x string) string {
		if p, ok := parent[x]; ok && p != x {
			r := find(p)
			parent[x] = r
			return r
		}
		parent[x] = x
		return x
	}
	for _, e := range topo.Edges {
		parent[find(e.From)] = find(e.To)
	}
	type domain struct {
		roots map[string][]string
		ids   []string
	}
	domains := map[string]*domain{}
	for id, s := range snaps {
		if s == nil || !s.STP.Enabled || s.STP.RootMAC == "" {
			continue
		}
		d := domains[find(id)]
		if d == nil {
			d = &domain{roots: map[string][]string{}}
			domains[find(id)] = d
		}
		d.roots[s.STP.RootMAC] = append(d.roots[s.STP.RootMAC], names[id])
		d.ids = append(d.ids, id)
	}
	var out []Issue
	for _, d := range domains {
		if len(d.roots) <= 1 {
			continue
		}
		var parts []string
		for mac, sws := range d.roots {
			sort.Strings(sws)
			parts = append(parts, fmt.Sprintf("%s (%s)", mac, strings.Join(sws, ", ")))
		}
		sort.Strings(parts)
		sort.Strings(d.ids)
		out = append(out, Issue{
			SwitchIDs: d.ids,
			Severity:  Warning, Category: "Spanning tree", SwitchName: "Network",
			Title:  fmt.Sprintf("%d different spanning-tree roots in one layer 2 network", len(d.roots)),
			Detail: "These switches are linked at layer 2 but don't agree on the root. Roots seen: " + strings.Join(parts, "; ") + ". Mixed MST regions or a link where spanning tree is off can cause this.",
		})
	}

	// Verify that each L2 domain with IGMP snooping active on a multicast/audio VLAN
	// also has at least one IGMP querier configured (or note when none is present).
	type mcastKey struct {
		dom string
		vid int
	}
	type mcastAgg struct {
		label      string
		snoopIDs   []string
		hasQuerier bool
	}
	mcastDomains := map[mcastKey]*mcastAgg{}
	for id, s := range snaps {
		if s == nil {
			continue
		}
		dscpPorts := map[string]bool{}
		for _, p := range s.Ports {
			if p.QoSTrust == "dscp" {
				dscpPorts[p.Name] = true
			}
		}
		dom := find(id)
		for _, v := range s.VLANs {
			if !v.IGMPConfigKnown || !v.IGMPSnooping || !isMulticastPriorityVLAN(v, dscpPorts) {
				continue
			}
			k := mcastKey{dom: dom, vid: v.ID}
			ag := mcastDomains[k]
			if ag == nil {
				ag = &mcastAgg{label: vlanLabel(v)}
				mcastDomains[k] = ag
			} else if len(v.Name) > 0 && ag.label == fmt.Sprintf("VLAN %d", v.ID) {
				ag.label = vlanLabel(v)
			}
			ag.snoopIDs = append(ag.snoopIDs, id)
			if v.IGMPQuerier {
				ag.hasQuerier = true
			}
		}
	}
	for _, ag := range mcastDomains {
		if ag.hasQuerier || len(ag.snoopIDs) == 0 {
			continue
		}
		sort.Strings(ag.snoopIDs)
		out = append(out, Issue{
			SwitchIDs: ag.snoopIDs,
			Severity:  Info, Category: "Multicast", SwitchName: "Network",
			Title:  fmt.Sprintf("No IGMP querier configured on %s", ag.label),
			Detail: "IGMP snooping is enabled on one or more switches, but none acts as IGMP querier in running-config. Unless an external router serves as querier, enable IGMP querier on one core switch for reliable DANTE/multicast.",
		})
	}

	// Audit switch-to-switch uplinks for asymmetric VLAN trunk mismatches:
	// when both connected switches define VLAN N (N != 1) but only one side
	// carries VLAN N on the inter-switch link (or its LAG/trunk).
	for _, e := range topo.Edges {
		sA, sB := snaps[e.From], snaps[e.To]
		if sA == nil || sB == nil || e.FromPort == "" || e.ToPort == "" {
			continue
		}
		if len(sA.VLANs) == 0 || len(sB.VLANs) == 0 {
			continue
		}
		carriedA := vlansOnUplinkPort(sA, e.FromPort)
		carriedB := vlansOnUplinkPort(sB, e.ToPort)
		defA := map[int]aruba.VLAN{}
		for _, v := range sA.VLANs {
			if v.ID > 1 {
				defA[v.ID] = v
			}
		}
		nA, nB := names[e.From], names[e.To]
		if nA == "" {
			nA = e.From
		}
		if nB == "" {
			nB = e.To
		}
		var mismatches []string
		for _, vB := range sB.VLANs {
			if vB.ID <= 1 {
				continue
			}
			vA, both := defA[vB.ID]
			if !both {
				continue
			}
			onA, onB := carriedA[vB.ID], carriedB[vB.ID]
			if onA == onB {
				continue
			}
			lbl := vlanLabel(vA)
			if vA.Name == "" && vB.Name != "" {
				lbl = vlanLabel(vB)
			}
			missing := nB + " port " + e.ToPort
			if !onA {
				missing = nA + " port " + e.FromPort
			}
			mismatches = append(mismatches, fmt.Sprintf("%s missing on %s", lbl, missing))
		}
		if len(mismatches) == 0 {
			continue
		}
		sort.Strings(mismatches)
		ids := []string{e.From, e.To}
		sort.Strings(ids)
		out = append(out, Issue{
			SwitchIDs: ids,
			Severity:  Warning, Category: "VLANs", SwitchName: "Network",
			Title:  fmt.Sprintf("Trunk VLAN mismatch on %s (%s) ↔ %s (%s)", nA, e.FromPort, nB, e.ToPort),
			Detail: "Both switches define these VLANs, but only one side of the uplink carries them: " + strings.Join(mismatches, "; ") + ". Tag the VLAN on both ends of the trunk.",
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Detail < out[j].Detail })
	return out
}

func isGigCapablePort(f aruba.Family, typ string) bool {
	tl := strings.ToLower(strings.TrimSpace(typ))
	if tl == "" {
		return f == aruba.FamilyAOSCX
	}
	return strings.Contains(tl, "1000") || strings.Contains(tl, "gig") || strings.Contains(tl, "sfp") ||
		strings.Contains(tl, "10g") || strings.Contains(tl, "2.5g") || strings.Contains(tl, "5g") ||
		strings.Contains(tl, "25g") || strings.Contains(tl, "40g") || strings.Contains(tl, "100g")
}

func vlansOnUplinkPort(snap *aruba.Snapshot, portName string) map[int]bool {
	aliases := map[string]bool{portName: true}
	for _, p := range snap.Ports {
		if p.Name == portName && p.Trunk != "" {
			aliases[p.Trunk] = true
		}
	}
	out := map[int]bool{}
	for _, v := range snap.VLANs {
		for _, p := range v.Untagged {
			if aliases[p] {
				out[v.ID] = true
			}
		}
		for _, p := range v.Tagged {
			if aliases[p] {
				out[v.ID] = true
			}
		}
	}
	return out
}

func vlanLabel(v aruba.VLAN) string {
	if v.Name != "" {
		return fmt.Sprintf("VLAN %d (%s)", v.ID, v.Name)
	}
	return fmt.Sprintf("VLAN %d", v.ID)
}

func isAudioDanteVLAN(v aruba.VLAN) bool {
	low := strings.ToLower(v.Name)
	if low == "" {
		return false
	}
	for _, sub := range []string{"dante", "sound", "audio", "aes67", "shure", "qsys"} {
		if strings.Contains(low, sub) {
			return true
		}
	}
	return false
}

func isMulticastPriorityVLAN(v aruba.VLAN, dscpPorts map[string]bool) bool {
	for _, p := range v.Untagged {
		if dscpPorts[p] {
			return true
		}
	}
	low := strings.ToLower(v.Name)
	if low == "" {
		return false
	}
	for _, sub := range []string{"sound", "dante", "audio", "aes67", "multicast", "iptv", "stream"} {
		if strings.Contains(low, sub) {
			return true
		}
	}
	for _, tok := range strings.FieldsFunc(low, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	}) {
		switch tok {
		case "ndi", "video", "voice", "voip", "av", "proav", "mcast":
			return true
		}
	}
	return false
}

func sortIssues(is []Issue) {
	sort.SliceStable(is, func(i, j int) bool {
		if is[i].Severity.rank() != is[j].Severity.rank() {
			return is[i].Severity.rank() < is[j].Severity.rank()
		}
		return is[i].SwitchName < is[j].SwitchName
	})
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func displayName(sw store.Switch, snap *aruba.Snapshot) string {
	if sw.Name != "" {
		return sw.Name
	}
	if snap != nil && snap.Facts.Hostname != "" {
		return snap.Facts.Hostname
	}
	return sw.Host
}

// opticIssues flags transceivers whose optical levels are out of range. Ports
// that are down with no light are skipped: an SFP in an empty port is normal.
func opticIssues(snap *aruba.Snapshot, add func(sev Severity, cat, title, detail string)) {
	up := map[string]bool{}
	known := map[string]bool{}
	for _, p := range snap.Ports {
		up[p.Name], known[p.Name] = p.Up, true
	}
	var crit, warn []string
	for _, o := range snap.Optics {
		level, reason := o.Judge()
		if level == "" {
			continue
		}
		rx, hasRx := o.WorstRx()
		if known[o.Port] && !up[o.Port] && (!hasRx || rx <= aruba.NoLight) {
			continue
		}
		d := "port " + o.Port
		if o.Type != "" {
			d += " (" + o.Type + ")"
		}
		if hasRx {
			d += fmt.Sprintf(": Rx %.1f dBm", rx)
			if o.RxLowWarn != nil && strings.Contains(reason, "below") {
				d += fmt.Sprintf(", warning at %.1f", *o.RxLowWarn)
			}
		}
		d += ", " + reason
		if level == "alarm" {
			crit = append(crit, d)
		} else {
			warn = append(warn, d)
		}
	}
	tip := ". Clean the connectors, check the patch cords and the optic on the far end."
	if len(crit) > 0 {
		add(Critical, "Optics", fmt.Sprintf("Optical power alarm on %d port(s)", len(crit)), strings.Join(crit, "; ")+tip)
	}
	if len(warn) > 0 {
		add(Warning, "Optics", fmt.Sprintf("Optical power warning on %d port(s)", len(warn)), strings.Join(warn, "; ")+tip)
	}
}

// localIsUTC says whether this computer's time zone is UTC or GMT for part
// of the year (London, Lisbon, Reykjavik...).
var localIsUTC = func() bool {
	y := time.Now().Year()
	for _, m := range []time.Month{time.January, time.July} {
		if _, off := time.Date(y, m, 1, 12, 0, 0, 0, time.Local).Zone(); off == 0 {
			return true
		}
	}
	return false
}
