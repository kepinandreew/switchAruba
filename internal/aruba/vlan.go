package aruba

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// VLANChange describes one VLAN edit coming from the UI.
type VLANChange struct {
	Action       string   `json:"action"` // create, update, delete
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	AddTagged    []string `json:"addTagged"`
	AddUntagged  []string `json:"addUntagged"`
	RemovePorts  []string `json:"removePorts"`
	IGMPSnooping *bool    `json:"igmpSnooping,omitempty"` // nil leaves IGMP snooping untouched
	IGMPQuerier  *bool    `json:"igmpQuerier,omitempty"`  // nil leaves IGMP querier untouched
	Save         bool     `json:"save"`
}

var safeName = regexp.MustCompile(`^[A-Za-z0-9 _.\-]{1,32}$`)
var safePort = regexp.MustCompile(`^((\d+/){0,2}[A-Za-z]{0,3}\d+|lag\d+|Trk\d+)$`)

func (c VLANChange) validate(current []VLAN) error {
	if c.ID < 1 || c.ID > 4094 {
		return fmt.Errorf("VLAN ID must be 1-4094")
	}
	switch c.Action {
	case "create", "update", "delete":
	default:
		return fmt.Errorf("unknown action %q", c.Action)
	}
	if c.Action == "delete" && c.ID == 1 {
		return fmt.Errorf("VLAN 1 can't be deleted")
	}
	if c.Name != "" && !safeName.MatchString(c.Name) {
		return fmt.Errorf("name may use letters, digits, space, _ . - (max 32)")
	}
	for _, list := range [][]string{c.AddTagged, c.AddUntagged, c.RemovePorts} {
		for _, p := range list {
			if !safePort.MatchString(p) {
				return fmt.Errorf("invalid port %q", p)
			}
		}
	}
	exists := false
	for _, v := range current {
		if v.ID == c.ID {
			exists = true
		}
	}
	if c.Action == "create" && exists {
		return fmt.Errorf("VLAN %d already exists", c.ID)
	}
	if (c.Action == "update" || c.Action == "delete") && !exists && len(current) > 0 {
		return fmt.Errorf("VLAN %d doesn't exist on this switch", c.ID)
	}
	return nil
}

// BuildVLANCommands returns the exact CLI lines that will be sent.
func BuildVLANCommands(f Family, c VLANChange, current []VLAN) ([]string, error) {
	if err := c.validate(current); err != nil {
		return nil, err
	}
	if f == FamilyInstantOn {
		return nil, fmt.Errorf("VLAN changes over SSH aren't supported on Instant On switches; use the Instant On portal or web UI")
	}
	cmds := []string{"configure terminal"}
	switch f {
	case FamilyAOSCX:
		if c.Action == "delete" {
			cmds = append(cmds, fmt.Sprintf("no vlan %d", c.ID))
			break
		}
		cmds = append(cmds, fmt.Sprintf("vlan %d", c.ID))
		if c.Name != "" {
			cmds = append(cmds, "name "+c.Name)
		}
		if c.IGMPSnooping != nil {
			if *c.IGMPSnooping {
				cmds = append(cmds, "ip igmp snooping enable")
			} else {
				cmds = append(cmds, "no ip igmp snooping enable")
			}
		}
		cmds = append(cmds, "exit")
		if c.IGMPQuerier != nil {
			cmds = append(cmds, fmt.Sprintf("interface vlan %d", c.ID))
			if *c.IGMPQuerier {
				cmds = append(cmds, "ip igmp enable", "ip igmp querier")
			} else {
				cmds = append(cmds, "no ip igmp querier")
			}
			cmds = append(cmds, "exit")
		}
		for _, p := range c.AddUntagged {
			cmds = append(cmds, "interface "+p, fmt.Sprintf("vlan access %d", c.ID), "exit")
		}
		for _, p := range c.AddTagged {
			cmds = append(cmds, "interface "+p, fmt.Sprintf("vlan trunk allowed %d", c.ID), "exit")
		}
		for _, p := range c.RemovePorts {
			if memberTagged(current, c.ID, p) {
				cmds = append(cmds, "interface "+p, fmt.Sprintf("no vlan trunk allowed %d", c.ID), "exit")
			} else {
				cmds = append(cmds, "interface "+p, "no vlan access", "exit")
			}
		}
	default: // AOS-S
		if c.Action == "delete" {
			cmds = append(cmds, fmt.Sprintf("no vlan %d", c.ID))
			break
		}
		cmds = append(cmds, fmt.Sprintf("vlan %d", c.ID))
		if c.Name != "" {
			cmds = append(cmds, fmt.Sprintf("name \"%s\"", c.Name))
		}
		if len(c.AddUntagged) > 0 {
			cmds = append(cmds, "untagged "+CompressPorts(c.AddUntagged))
		}
		if len(c.AddTagged) > 0 {
			cmds = append(cmds, "tagged "+CompressPorts(c.AddTagged))
		}
		var rmT, rmU []string
		for _, p := range c.RemovePorts {
			if memberTagged(current, c.ID, p) {
				rmT = append(rmT, p)
			} else {
				rmU = append(rmU, p)
			}
		}
		if len(rmT) > 0 {
			cmds = append(cmds, "no tagged "+CompressPorts(rmT))
		}
		if len(rmU) > 0 {
			cmds = append(cmds, "no untagged "+CompressPorts(rmU))
		}
		if c.IGMPSnooping != nil {
			if *c.IGMPSnooping {
				cmds = append(cmds, "ip igmp")
			} else {
				cmds = append(cmds, "no ip igmp")
			}
		}
		if c.IGMPQuerier != nil && !(c.IGMPSnooping != nil && !*c.IGMPSnooping) {
			if *c.IGMPQuerier {
				cmds = append(cmds, "ip igmp querier")
			} else {
				cmds = append(cmds, "no ip igmp querier")
			}
		}
		cmds = append(cmds, "exit")
	}
	cmds = append(cmds, "end")
	if c.Save {
		cmds = append(cmds, "write memory")
	}
	return cmds, nil
}

// PortVLANChange describes changing VLAN mode (access or trunk) and membership on a set of ports.
type PortVLANChange struct {
	Ports       []string `json:"ports"`
	Mode        string   `json:"mode"`               // "access" or "trunk"
	AccessVLAN  int      `json:"accessVlan"`         // Access VLAN (or native/untagged VLAN in trunk mode)
	TaggedVLANs []int    `json:"taggedVlans"`        // Tagged VLANs allowed on trunk
	TagAll      bool     `json:"tagAll"`             // Trunk allowed all VLANs
	QoSTrust    string   `json:"qosTrust,omitempty"` // "", "dscp", "cos", "none", "default"
	DisableEEE  *bool    `json:"disableEee,omitempty"`
	Save        bool     `json:"save"`
}

// BuildPortVLANCommands returns the CLI lines to configure VLAN on the specified ports.
func BuildPortVLANCommands(f Family, c PortVLANChange, current []VLAN) ([]string, error) {
	if len(c.Ports) == 0 {
		return nil, fmt.Errorf("no ports specified")
	}
	for _, p := range c.Ports {
		if !safePort.MatchString(p) {
			return nil, fmt.Errorf("invalid port %q", p)
		}
	}
	if c.Mode != "access" && c.Mode != "trunk" {
		return nil, fmt.Errorf("unknown mode %q (must be access or trunk)", c.Mode)
	}
	if c.AccessVLAN < 1 || c.AccessVLAN > 4094 {
		return nil, fmt.Errorf("VLAN ID must be 1-4094")
	}
	for _, v := range c.TaggedVLANs {
		if v < 1 || v > 4094 {
			return nil, fmt.Errorf("tagged VLAN ID must be 1-4094")
		}
	}
	if f == FamilyInstantOn {
		return nil, fmt.Errorf("VLAN changes over SSH aren't supported on Instant On switches; use the Instant On portal or web UI")
	}
	switch c.QoSTrust {
	case "", "dscp", "cos", "none", "default":
	default:
		return nil, fmt.Errorf("unknown qosTrust %q (expected dscp, cos, none, or default)", c.QoSTrust)
	}

	hasVLAN := func(id int) bool {
		for _, v := range current {
			if v.ID == id {
				return true
			}
		}
		return false
	}

	cmds := []string{"configure terminal"}
	switch f {
	case FamilyAOSCX:
		needed := []int{c.AccessVLAN}
		if c.Mode == "trunk" {
			needed = append(needed, c.TaggedVLANs...)
		}
		for _, vid := range needed {
			if vid > 0 && !hasVLAN(vid) {
				cmds = append(cmds, fmt.Sprintf("vlan %d", vid), "exit")
			}
		}
		for _, p := range c.Ports {
			cmds = append(cmds, "interface "+p)
			if c.Mode == "access" {
				// If port had tagged VLANs previously, clean them up
				var oldTagged []int
				for _, v := range current {
					for _, tp := range v.Tagged {
						if tp == p {
							oldTagged = append(oldTagged, v.ID)
						}
					}
				}
				if len(oldTagged) > 0 {
					cmds = append(cmds, fmt.Sprintf("no vlan trunk allowed %s", compressInts(oldTagged)))
					cmds = append(cmds, "no vlan trunk native")
				}
				cmds = append(cmds, fmt.Sprintf("vlan access %d", c.AccessVLAN))
			} else {
				// Trunk mode
				if c.AccessVLAN > 0 {
					cmds = append(cmds, fmt.Sprintf("vlan trunk native %d", c.AccessVLAN))
				}
				if c.TagAll {
					cmds = append(cmds, "vlan trunk allowed all")
				} else if len(c.TaggedVLANs) > 0 {
					allowed := append([]int(nil), c.TaggedVLANs...)
					if c.AccessVLAN > 0 {
						hasNative := false
						for _, av := range allowed {
							if av == c.AccessVLAN {
								hasNative = true
								break
							}
						}
						if !hasNative {
							allowed = append([]int{c.AccessVLAN}, allowed...)
						}
					}
					sort.Ints(allowed)
					cmds = append(cmds, fmt.Sprintf("vlan trunk allowed %s", compressInts(allowed)))
				}
			}
			switch c.QoSTrust {
			case "dscp":
				cmds = append(cmds, "qos trust dscp")
			case "cos":
				cmds = append(cmds, "qos trust cos")
			case "none":
				cmds = append(cmds, "qos trust none")
			case "default":
				cmds = append(cmds, "no qos trust")
			}
			if c.DisableEEE != nil {
				if *c.DisableEEE {
					cmds = append(cmds, "no energy-efficient-ethernet")
				} else {
					cmds = append(cmds, "energy-efficient-ethernet")
				}
			}
			cmds = append(cmds, "exit")
		}

	default: // AOS-S
		if !hasVLAN(c.AccessVLAN) {
			cmds = append(cmds, fmt.Sprintf("vlan %d", c.AccessVLAN), "exit")
		}
		if c.Mode == "access" {
			// If ports were tagged in other VLANs, remove them from tagged
			for _, v := range current {
				if v.ID == c.AccessVLAN {
					continue
				}
				var rmTagged []string
				for _, tp := range v.Tagged {
					for _, cp := range c.Ports {
						if tp == cp {
							rmTagged = append(rmTagged, cp)
						}
					}
				}
				if len(rmTagged) > 0 {
					cmds = append(cmds, fmt.Sprintf("vlan %d", v.ID), "no tagged "+CompressPorts(rmTagged), "exit")
				}
			}
			cmds = append(cmds, fmt.Sprintf("vlan %d", c.AccessVLAN))
			cmds = append(cmds, "untagged "+CompressPorts(c.Ports))
			cmds = append(cmds, "exit")
		} else {
			// Trunk mode on AOS-S
			if c.AccessVLAN > 0 {
				cmds = append(cmds, fmt.Sprintf("vlan %d", c.AccessVLAN))
				cmds = append(cmds, "untagged "+CompressPorts(c.Ports))
				cmds = append(cmds, "exit")
			}
			for _, vid := range c.TaggedVLANs {
				if !hasVLAN(vid) {
					cmds = append(cmds, fmt.Sprintf("vlan %d", vid), "exit")
				}
				cmds = append(cmds, fmt.Sprintf("vlan %d", vid))
				cmds = append(cmds, "tagged "+CompressPorts(c.Ports))
				cmds = append(cmds, "exit")
			}
			// Clean up tagged VLANs no longer in c.TaggedVLANs
			for _, v := range current {
				if v.ID == c.AccessVLAN {
					continue
				}
				isStillTagged := false
				for _, tv := range c.TaggedVLANs {
					if tv == v.ID {
						isStillTagged = true
						break
					}
				}
				if isStillTagged {
					continue
				}
				var rmTagged []string
				for _, tp := range v.Tagged {
					for _, cp := range c.Ports {
						if tp == cp {
							rmTagged = append(rmTagged, cp)
						}
					}
				}
				if len(rmTagged) > 0 {
					cmds = append(cmds, fmt.Sprintf("vlan %d", v.ID), "no tagged "+CompressPorts(rmTagged), "exit")
				}
			}
		}
		switch c.QoSTrust {
		case "dscp":
			cmds = append(cmds, "qos type-of-service diff-services")
		case "none", "default":
			cmds = append(cmds, "no qos type-of-service")
		case "cos":
			return nil, fmt.Errorf("AOS-S switches do not have per-port 'qos trust cos'; choose DSCP (DiffServ) or leave QoS unchanged")
		}
		if c.DisableEEE != nil {
			cmds = append(cmds, "interface "+CompressPorts(c.Ports))
			if *c.DisableEEE {
				cmds = append(cmds, "no energy-efficient-ethernet")
			} else {
				cmds = append(cmds, "energy-efficient-ethernet")
			}
			cmds = append(cmds, "exit")
		}
	}

	cmds = append(cmds, "end")
	if c.Save {
		cmds = append(cmds, "write memory")
	}
	return cmds, nil
}

// AfterPortVLANPorts updates per-port QoS trust and EEE state in memory
// immediately after a PortVLANChange succeeds.
func AfterPortVLANPorts(ports []Port, f Family, c PortVLANChange) []Port {
	if (c.QoSTrust == "" || f != FamilyAOSCX) && c.DisableEEE == nil {
		return ports
	}
	portSet := map[string]bool{}
	for _, p := range c.Ports {
		portSet[p] = true
	}
	out := make([]Port, len(ports))
	copy(out, ports)
	val := c.QoSTrust
	if val == "default" {
		val = ""
	}
	for i := range out {
		if portSet[out[i].Name] {
			if c.QoSTrust != "" && f == FamilyAOSCX {
				out[i].QoSTrust = val
			}
			if c.DisableEEE != nil {
				out[i].EEEOff = *c.DisableEEE
				out[i].EEEOn = !*c.DisableEEE
			}
		}
	}
	return out
}

// AfterPortVLANChange updates the in-memory VLAN table immediately after a PortVLANChange succeeds.
func AfterPortVLANChange(vlans []VLAN, c PortVLANChange) []VLAN {
	portSet := map[string]bool{}
	for _, p := range c.Ports {
		portSet[p] = true
	}
	tagSet := map[int]bool{}
	if c.Mode == "trunk" {
		for _, v := range c.TaggedVLANs {
			tagSet[v] = true
		}
	}
	without := func(ports []string) []string {
		var out []string
		for _, p := range ports {
			if !portSet[p] {
				out = append(out, p)
			}
		}
		return out
	}
	var out []VLAN
	hasAccess := false
	seenTag := map[int]bool{}
	for _, v := range vlans {
		v.Untagged = without(v.Untagged)
		v.Tagged = without(v.Tagged)
		if v.ID == c.AccessVLAN && c.AccessVLAN > 0 {
			hasAccess = true
			v.Untagged = append(v.Untagged, c.Ports...)
		} else if c.Mode == "trunk" && (c.TagAll || tagSet[v.ID]) {
			seenTag[v.ID] = true
			v.Tagged = append(v.Tagged, c.Ports...)
		}
		out = append(out, v)
	}
	if c.AccessVLAN > 0 && !hasAccess {
		out = append(out, VLAN{ID: c.AccessVLAN, Untagged: append([]string(nil), c.Ports...)})
	}
	if c.Mode == "trunk" && !c.TagAll {
		for _, tv := range c.TaggedVLANs {
			if tv != c.AccessVLAN && !seenTag[tv] {
				out = append(out, VLAN{ID: tv, Tagged: append([]string(nil), c.Ports...)})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// PortPoEChange describes turning PoE on, off, or power-cycling (bounce) PoE on a set of ports.
type PortPoEChange struct {
	Ports  []string `json:"ports"`
	Action string   `json:"action"` // "on", "off", or "bounce"
	Save   bool     `json:"save"`
}

// BuildPortPoECommands returns the CLI lines to toggle or bounce PoE on the specified ports.
func BuildPortPoECommands(f Family, c PortPoEChange) ([]string, error) {
	if len(c.Ports) == 0 {
		return nil, fmt.Errorf("no ports specified")
	}
	for _, p := range c.Ports {
		if !safePort.MatchString(p) {
			return nil, fmt.Errorf("invalid port %q", p)
		}
	}
	if c.Action != "on" && c.Action != "off" && c.Action != "bounce" {
		return nil, fmt.Errorf("unknown PoE action %q (must be on, off, or bounce)", c.Action)
	}
	if f == FamilyInstantOn {
		return nil, fmt.Errorf("PoE changes over SSH aren't supported on Instant On switches; use the Instant On portal or web UI")
	}

	cmds := []string{"configure terminal"}
	switch f {
	case FamilyAOSCX:
		for _, p := range c.Ports {
			cmds = append(cmds, "interface "+p)
			switch c.Action {
			case "on":
				cmds = append(cmds, "power-over-ethernet")
			case "off":
				cmds = append(cmds, "no power-over-ethernet")
			case "bounce":
				cmds = append(cmds, "no power-over-ethernet", "power-over-ethernet")
			}
			cmds = append(cmds, "exit")
		}
	default: // AOS-S
		cmds = append(cmds, "interface "+CompressPorts(c.Ports))
		switch c.Action {
		case "on":
			cmds = append(cmds, "power-over-ethernet")
		case "off":
			cmds = append(cmds, "no power-over-ethernet")
		case "bounce":
			cmds = append(cmds, "no power-over-ethernet", "power-over-ethernet")
		}
		cmds = append(cmds, "exit")
	}
	cmds = append(cmds, "end")
	if c.Save {
		cmds = append(cmds, "write memory")
	}
	return cmds, nil
}

// AfterPortPoEChange updates the in-memory Port list immediately after a PortPoEChange succeeds.
func AfterPortPoEChange(ports []Port, c PortPoEChange) []Port {
	portSet := map[string]bool{}
	for _, p := range c.Ports {
		portSet[p] = true
	}
	out := make([]Port, len(ports))
	copy(out, ports)
	for i := range out {
		if portSet[out[i].Name] {
			out[i].PoEOff = (c.Action == "off")
		}
	}
	return out
}

// PortAdminChange describes enabling, disabling (shutdown), or bouncing the admin state of ports.
type PortAdminChange struct {
	Ports  []string `json:"ports"`
	Action string   `json:"action"` // "enable", "disable", or "bounce"
	Save   bool     `json:"save"`
}

// BuildPortAdminCommands returns the CLI lines to enable, disable, or bounce the specified ports.
func BuildPortAdminCommands(f Family, c PortAdminChange) ([]string, error) {
	if len(c.Ports) == 0 {
		return nil, fmt.Errorf("no ports specified")
	}
	for _, p := range c.Ports {
		if !safePort.MatchString(p) {
			return nil, fmt.Errorf("invalid port %q", p)
		}
	}
	if c.Action != "enable" && c.Action != "disable" && c.Action != "bounce" {
		return nil, fmt.Errorf("unknown port action %q (must be enable, disable, or bounce)", c.Action)
	}
	if f == FamilyInstantOn {
		return nil, fmt.Errorf("port admin changes over SSH aren't supported on Instant On switches; use the Instant On portal or web UI")
	}

	cmds := []string{"configure terminal"}
	switch f {
	case FamilyAOSCX:
		for _, p := range c.Ports {
			cmds = append(cmds, "interface "+p)
			switch c.Action {
			case "enable":
				cmds = append(cmds, "no shutdown")
			case "disable":
				cmds = append(cmds, "shutdown")
			case "bounce":
				cmds = append(cmds, "shutdown", "no shutdown")
			}
			cmds = append(cmds, "exit")
		}
	default: // AOS-S
		cmds = append(cmds, "interface "+CompressPorts(c.Ports))
		switch c.Action {
		case "enable":
			cmds = append(cmds, "enable")
		case "disable":
			cmds = append(cmds, "disable")
		case "bounce":
			cmds = append(cmds, "disable", "enable")
		}
		cmds = append(cmds, "exit")
	}
	cmds = append(cmds, "end")
	if c.Save {
		cmds = append(cmds, "write memory")
	}
	return cmds, nil
}

// AfterPortAdminChange updates the in-memory Port list immediately after a PortAdminChange succeeds.
func AfterPortAdminChange(ports []Port, c PortAdminChange) []Port {
	portSet := map[string]bool{}
	for _, p := range c.Ports {
		portSet[p] = true
	}
	out := make([]Port, len(ports))
	copy(out, ports)
	for i := range out {
		if portSet[out[i].Name] {
			if c.Action == "disable" {
				out[i].Enabled = false
				out[i].Up = false
			} else {
				out[i].Enabled = true
			}
		}
	}
	return out
}

// AfterChange returns the VLAN table as it should be once c has been accepted,
// so the UI shows the change before the switch is read again. A port can only
// be untagged in one VLAN, so adding it untagged takes it out of the others.
func AfterChange(vlans []VLAN, c VLANChange) []VLAN {
	without := func(ports []string, drop map[string]bool) []string {
		var out []string
		for _, p := range ports {
			if !drop[p] {
				out = append(out, p)
			}
		}
		return out
	}
	set := func(ports []string) map[string]bool {
		m := map[string]bool{}
		for _, p := range ports {
			m[p] = true
		}
		return m
	}
	untagged, removed := set(c.AddUntagged), set(c.RemovePorts)
	var out []VLAN
	found := false
	for _, v := range vlans {
		if v.ID == c.ID {
			if c.Action == "delete" {
				continue
			}
			found = true
			if c.Name != "" {
				v.Name = c.Name
			}
			tagged := set(c.AddTagged)
			// a port added again isn't listed twice
			v.Untagged = append(without(without(v.Untagged, removed), untagged), c.AddUntagged...)
			v.Tagged = append(without(without(v.Tagged, removed), tagged), c.AddTagged...)
			if c.IGMPSnooping != nil {
				v.IGMPSnooping = *c.IGMPSnooping
				v.IGMPConfigKnown = true
				if !v.IGMPSnooping {
					v.IGMPQuerier = false
				}
			}
			if c.IGMPQuerier != nil && (c.IGMPSnooping == nil || *c.IGMPSnooping) {
				v.IGMPQuerier = *c.IGMPQuerier
				v.IGMPConfigKnown = true
			}
		} else {
			v.Untagged = without(v.Untagged, untagged)
		}
		out = append(out, v)
	}
	if c.Action == "create" && !found {
		nv := VLAN{ID: c.ID, Name: c.Name, Untagged: c.AddUntagged, Tagged: c.AddTagged}
		if c.IGMPSnooping != nil {
			nv.IGMPSnooping = *c.IGMPSnooping
			nv.IGMPConfigKnown = true
		}
		if c.IGMPQuerier != nil && nv.IGMPSnooping {
			nv.IGMPQuerier = *c.IGMPQuerier
			nv.IGMPConfigKnown = true
		}
		out = append(out, nv)
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	}
	return out
}

func memberTagged(vlans []VLAN, id int, port string) bool {
	for _, v := range vlans {
		if v.ID == id {
			for _, p := range v.Tagged {
				if p == port {
					return true
				}
			}
		}
	}
	return false
}

// ApplyResult is the transcript of a change.
type ApplyResult struct {
	OK         bool     `json:"ok"`
	Transcript []string `json:"transcript"`
	Error      string   `json:"error"`
}

// ApplyCommands sends config lines one by one and stops at the first rejection.
// "write memory" is only reached if every previous line was accepted.
func ApplyCommands(ctx context.Context, r Runner, cmds []string) ApplyResult {
	res := ApplyResult{}
	for _, c := range cmds {
		out, err := r.Run(ctx, c, 30*time.Second)
		res.Transcript = append(res.Transcript, "> "+c)
		if strings.TrimSpace(out) != "" {
			res.Transcript = append(res.Transcript, out)
		}
		if err != nil {
			res.Error = fmt.Sprintf("%s: %v", c, err)
			r.Run(ctx, "end", 5*time.Second)
			return res
		}
		if IsCLIError(out) || strings.Contains(strings.ToLower(out), "error") && !strings.Contains(strings.ToLower(out), "0 error") {
			res.Error = fmt.Sprintf("switch rejected %q", c)
			r.Run(ctx, "end", 5*time.Second)
			return res
		}
	}
	res.OK = true
	return res
}
