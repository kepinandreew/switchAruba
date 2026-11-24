package aruba

import (
	"fmt"
	"sort"
)

// DanteProfileChange applies Audinate/Aruba Dante AV best practices to a switch
// transparently: DSCP trust, DSCP-to-queue priority mapping (56 PTP -> 7, 46 Audio -> 6),
// Strict Priority queue scheduling (AOS-CX), IGMP snooping (+ optional querier), and
// disabling Energy-Efficient Ethernet (EEE) on audio ports.
type DanteProfileChange struct {
	VLANIDs        []int    `json:"vlanIds"`
	Ports          []string `json:"ports"`
	GlobalTrust    bool     `json:"globalTrust"`
	PortTrust      bool     `json:"portTrust,omitempty"`
	DSCPMap        bool     `json:"dscpMap"`
	StrictSchedule bool     `json:"strictSchedule"`
	IGMPSnooping   bool     `json:"igmpSnooping"`
	IGMPQuerier    bool     `json:"igmpQuerier"`
	DisableEEE     bool     `json:"disableEee"`
	JumboMTU       bool     `json:"jumboMtu,omitempty"`
	Save           bool     `json:"save"`
}

// BuildDanteProfileCommands generates Aruba AOS-CX or AOS-S CLI lines for a Dante AV profile.
func BuildDanteProfileCommands(f Family, c DanteProfileChange) ([]string, error) {
	if f == FamilyInstantOn {
		return nil, fmt.Errorf("Dante AV profile changes over SSH aren't supported on Instant On switches; use the Instant On portal or web UI")
	}
	if !c.GlobalTrust && !c.PortTrust && !c.DSCPMap && !c.StrictSchedule && !c.IGMPSnooping && !c.IGMPQuerier && !c.DisableEEE && !c.JumboMTU {
		return nil, fmt.Errorf("select at least one Dante AV profile setting to apply")
	}
	for _, p := range c.Ports {
		if !safePort.MatchString(p) {
			return nil, fmt.Errorf("invalid port %q", p)
		}
	}
	vlans := dedupSortedInts(c.VLANIDs)
	for _, vid := range vlans {
		if vid < 1 || vid > 4094 {
			return nil, fmt.Errorf("VLAN ID %d must be 1-4094", vid)
		}
	}
	if (c.IGMPSnooping || c.IGMPQuerier) && len(vlans) == 0 {
		return nil, fmt.Errorf("select at least one VLAN when enabling IGMP snooping or querier")
	}

	cmds := []string{"configure terminal"}
	switch f {
	case FamilyAOSCX:
		if c.GlobalTrust {
			cmds = append(cmds, "qos trust dscp")
		}
		if c.DSCPMap {
			cmds = append(cmds,
				"qos dscp-map 56 local-priority 7 name Dante_PTP",
				"qos dscp-map 46 local-priority 6 name Dante_Audio",
			)
		}
		if c.StrictSchedule {
			cmds = append(cmds,
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
		}
		if c.IGMPSnooping {
			for _, vid := range vlans {
				cmds = append(cmds, fmt.Sprintf("vlan %d", vid), "ip igmp snooping enable", "exit")
			}
		}
		if c.IGMPQuerier {
			for _, vid := range vlans {
				cmds = append(cmds, fmt.Sprintf("interface vlan %d", vid), "ip igmp enable", "ip igmp querier", "exit")
			}
		}
		for _, p := range c.Ports {
			needPortTrust := c.PortTrust
			if !needPortTrust && !c.DisableEEE && !c.JumboMTU {
				continue
			}
			cmds = append(cmds, "interface "+p)
			if needPortTrust {
				cmds = append(cmds, "qos trust dscp")
			}
			if c.DisableEEE {
				cmds = append(cmds, "no energy-efficient-ethernet")
			}
			if c.JumboMTU {
				cmds = append(cmds, "mtu 9198")
			}
			cmds = append(cmds, "exit")
		}

	default: // AOS-S
		if c.GlobalTrust || c.PortTrust {
			cmds = append(cmds, "qos type-of-service diff-services")
		}
		if c.DSCPMap {
			cmds = append(cmds,
				"qos dscp-map 111000 priority 7",
				"qos dscp-map 101110 priority 6",
			)
		}
		if c.IGMPSnooping || c.IGMPQuerier || c.JumboMTU {
			targetVLANs := vlans
			if len(targetVLANs) == 0 && c.JumboMTU {
				targetVLANs = []int{1}
			}
			for _, vid := range targetVLANs {
				cmds = append(cmds, fmt.Sprintf("vlan %d", vid))
				if c.IGMPSnooping {
					cmds = append(cmds, "ip igmp")
				}
				if c.IGMPQuerier {
					cmds = append(cmds, "ip igmp querier")
				}
				if c.JumboMTU {
					cmds = append(cmds, "jumbo")
				}
				cmds = append(cmds, "exit")
			}
		}
		if c.DisableEEE && len(c.Ports) > 0 {
			cmds = append(cmds, "interface "+CompressPorts(c.Ports), "no energy-efficient-ethernet", "exit")
		}
	}

	cmds = append(cmds, "end")
	if c.Save {
		cmds = append(cmds, "write memory")
	}
	return cmds, nil
}

// AfterDanteProfileChange updates in-memory snapshot fields right after a Dante profile succeeds.
func AfterDanteProfileChange(snap *Snapshot, c DanteProfileChange) *Snapshot {
	if snap == nil {
		return nil
	}
	next := *snap
	qos := QoSInfo{Known: true, DSCP56Priority: -1, DSCP46Priority: -1}
	if snap.QoS != nil {
		qos = *snap.QoS
		qos.Known = true
	}
	if snap.Family == FamilyAOSS {
		qos.StrictTopQueues = true
	}
	if c.GlobalTrust || (snap.Family == FamilyAOSS && c.PortTrust) {
		qos.GlobalTrust = "dscp"
	}
	if c.DSCPMap {
		qos.DSCP56Priority = 7
		qos.DSCP46Priority = 6
	}
	if c.StrictSchedule && snap.Family == FamilyAOSCX {
		qos.ScheduleProfile = "dante_strict"
		qos.StrictTopQueues = true
	}
	next.QoS = &qos

	if c.IGMPSnooping || c.IGMPQuerier {
		vset := map[int]bool{}
		for _, vid := range c.VLANIDs {
			vset[vid] = true
		}
		vlans := make([]VLAN, len(snap.VLANs))
		copy(vlans, snap.VLANs)
		for i := range vlans {
			if vset[vlans[i].ID] {
				if c.IGMPSnooping {
					vlans[i].IGMPSnooping = true
					vlans[i].IGMPConfigKnown = true
				}
				if c.IGMPQuerier {
					vlans[i].IGMPQuerier = true
					vlans[i].IGMPConfigKnown = true
				}
			}
		}
		next.VLANs = vlans
	}

	if len(c.Ports) > 0 {
		pset := map[string]bool{}
		for _, p := range c.Ports {
			pset[p] = true
		}
		ports := make([]Port, len(snap.Ports))
		copy(ports, snap.Ports)
		for i := range ports {
			if pset[ports[i].Name] {
				if snap.Family == FamilyAOSCX && c.PortTrust {
					ports[i].QoSTrust = "dscp"
				}
				if c.DisableEEE {
					ports[i].EEEOff = true
					ports[i].EEEOn = false
				}
			}
		}
		next.Ports = ports
	}
	return &next
}

func dedupSortedInts(in []int) []int {
	if len(in) == 0 {
		return nil
	}
	seen := map[int]bool{}
	var out []int
	for _, x := range in {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Ints(out)
	return out
}
