package core

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"myswitch/internal/aruba"
	"myswitch/internal/store"
)

func TestTopologyAndIssues(t *testing.T) {
	core := store.Switch{ID: "a", Host: "10.0.0.1", Port: 22}
	acc := store.Switch{ID: "b", Host: "10.0.0.11", Port: 22}
	coreSnap := &aruba.Snapshot{
		Family: aruba.FamilyAOSCX, CollectedAt: time.Now(),
		Facts: aruba.Facts{Hostname: "core-cx", BaseMAC: "00:11:22:aa:bb:cc", CPU: 5, Memory: 20},
		STP:   aruba.STP{Known: true, Enabled: true, IsRoot: true, BridgePriority: "4096", RootMAC: "00:11:22:aa:bb:cc"},
		Neighbors: []aruba.Neighbor{
			{Protocol: "lldp", LocalPort: "1/1/1", SysName: "SW-ACCESS-01", MgmtIP: "10.0.0.11", PortID: "49"},
			{Protocol: "lldp", LocalPort: "1/1/48", SysName: "fw01", Description: "FortiGate-100F", Capabilities: "Router"},
		},
		RunningConfig: "x", Security: aruba.Security{NTPServers: []string{"1"}, LogServers: []string{"1"}, AAA: []string{"x"}},
	}
	accSnap := &aruba.Snapshot{
		Family: aruba.FamilyAOSS, CollectedAt: time.Now(),
		Facts: aruba.Facts{Hostname: "SW-ACCESS-01", CPU: 95, Memory: 10},
		STP:   aruba.STP{Known: true, Enabled: true, RootMAC: "00:11:22:aa:bb:cc", Ports: []aruba.STPPort{{Port: "50", State: "Blocking"}}},
		Neighbors: []aruba.Neighbor{
			{Protocol: "lldp", LocalPort: "49", SysName: "core-cx", ChassisID: "00 11 22 aa bb cc", PortID: "1/1/1"},
			{Protocol: "cdp", LocalPort: "49", SysName: "core-cx", PortID: "1/1/1"},
			{Protocol: "lldp", LocalPort: "5", SysName: "SEP00AABBCCDDEE", Description: "Cisco IP Phone"},
		},
		Security:      aruba.Security{TelnetEnabled: true, HTTPPlain: true, SNMPDefault: true},
		RunningConfig: "x",
		Env:           []aruba.EnvItem{{Kind: "fan", Name: "Sys-2", Status: "Fan Failed", OK: false}},
	}
	infos := []switchInfo{{sw: core, snap: coreSnap, online: true}, {sw: acc, snap: accSnap, online: true}}
	topo := buildTopology(infos, map[string]store.Point{"a": {X: 1, Y: 2}}, false)
	if len(topo.Nodes) != 3 { // core, access, firewall (phone hidden)
		t.Fatalf("nodes: %+v", topo.Nodes)
	}
	if len(topo.Edges) != 2 {
		t.Fatalf("edges: %+v", topo.Edges)
	}
	for _, e := range topo.Edges {
		if e.From == "a" && e.To == "b" && (e.FromPort != "1/1/1" || e.ToPort != "49" || len(e.Protocols) != 2) {
			t.Fatalf("core link: %+v", e)
		}
	}
	if topo.Nodes[0].X == nil || *topo.Nodes[0].X != 1 {
		t.Fatal("layout not applied")
	}
	withPhones := buildTopology(infos, nil, true)
	if len(withPhones.Nodes) != 4 {
		t.Fatalf("with endpoints: %d", len(withPhones.Nodes))
	}

	st := &Status{Online: true, Checked: true, TopologyDelta: 3}
	is := evaluate(acc, st, accSnap, store.DefaultSettings())
	want := map[string]bool{"Telnet is enabled": false, "Default SNMP community in use": false, "Fan problem": false, "High CPU: 95%": false, "3 topology change(s) since last check": false, "No manager password set": false}
	for _, i := range is {
		if _, ok := want[i.Title]; ok {
			want[i.Title] = true
		}
	}
	for k, v := range want {
		if !v {
			t.Errorf("missing issue %q in %+v", k, is)
		}
	}
	off := evaluate(core, &Status{Checked: true, Online: false, PingError: "timeout"}, nil, store.DefaultSettings())
	if len(off) != 1 || off[0].Severity != Critical {
		t.Fatalf("offline: %+v", off)
	}
	two := map[string]*aruba.Snapshot{"a": coreSnap, "b": {STP: aruba.STP{Enabled: true, RootMAC: "ff:ff:ff:00:00:01"}}}
	names := map[string]string{"a": "core", "b": "acc"}
	linked := Topology{Edges: []Edge{{From: "a", To: "b"}}}
	if split := networkIssues(two, names, linked); len(split) != 1 {
		t.Fatal("expected split-root issue")
	} else if !reflect.DeepEqual(split[0].SwitchIDs, []string{"a", "b"}) {
		// the site filter places the issue by these
		t.Fatalf("switch ids: %v", split[0].SwitchIDs)
	}
	// linked only through an unmanaged switch still counts as one layer 2 network
	viaExt := Topology{Edges: []Edge{{From: "a", To: "ext:x"}, {From: "b", To: "ext:x"}}}
	if split := networkIssues(two, names, viaExt); len(split) != 1 {
		t.Fatal("expected split-root issue through an unmanaged switch")
	}
	// branches joined by routing: a root each is normal
	if split := networkIssues(two, names, Topology{}); len(split) != 0 {
		t.Fatalf("routed branches flagged: %+v", split)
	}
}

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.SaveCredential(store.Credential{Name: "default", Username: "admin", Password: "p@ss"})
	if err != nil || c.Password != "" {
		t.Fatal(err, c)
	}
	sw, err := s.SaveSwitch(store.Switch{Host: "10.0.0.1", CredentialID: c.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveSwitch(store.Switch{Host: "10.0.0.1"}); err == nil {
		t.Fatal("duplicate accepted")
	}
	s2, _ := store.Open(dir)
	got, err := s2.Credential(c.ID)
	if err != nil || got.Password != "p@ss" {
		t.Fatal("password roundtrip", err, got)
	}
	if err := s2.DeleteCredential(c.ID); err == nil {
		t.Fatal("deleted credential in use")
	}
	saved, _ := s2.SaveBackup(sw.ID, "hostname a\nvlan 1", 3)
	again, _ := s2.SaveBackup(sw.ID, "hostname a\nvlan 1", 3)
	if !saved || again {
		t.Fatal("backup dedupe", saved, again)
	}
}

func TestDisabledSwitch(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, _ := st.SaveSwitch(store.Switch{Host: "10.9.9.1"})
	b, _ := st.SaveSwitch(store.Switch{Host: "10.9.9.2"})
	m := New(st, nil)
	m.status[a.ID] = &Status{Checked: true, Online: false}
	m.status[b.ID] = &Status{Checked: true, Online: false}
	if d := m.Dashboard(); d.Offline != 2 || d.Critical != 2 {
		t.Fatalf("before: %+v", d)
	}
	st.SetDisabled([]string{b.ID}, true)
	d := m.Dashboard()
	if d.Offline != 1 || d.Disabled != 1 || d.Critical != 1 || d.Total != 2 {
		t.Fatalf("after: offline=%d disabled=%d critical=%d", d.Offline, d.Disabled, d.Critical)
	}
	m.Enqueue(b.ID)
	if m.stat(b.ID).Collecting {
		t.Fatal("paused switch was queued")
	}
}

func TestVLANOverviewEditable(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cx, _ := st.SaveSwitch(store.Switch{Host: "10.9.9.1"})
	paused, _ := st.SaveSwitch(store.Switch{Host: "10.9.9.2"})
	web, _ := st.SaveSwitch(store.Switch{Host: "10.9.9.3", Web: true})
	st.SetDisabled([]string{paused.ID}, true)
	m := New(st, nil)
	m.snaps[cx.ID] = &aruba.Snapshot{Family: aruba.FamilyAOSCX, VLANs: []aruba.VLAN{{ID: 1}}}
	m.snaps[paused.ID] = &aruba.Snapshot{Family: aruba.FamilyAOSS}
	m.snaps[web.ID] = &aruba.Snapshot{Family: aruba.FamilyInstantOn}
	got := map[string]SwitchVLANs{}
	for _, s := range m.VLANOverview() {
		got[s.ID] = s
	}
	if !got[cx.ID].Editable || got[cx.ID].Family != "AOS-CX" || len(got[cx.ID].VLANs) != 1 {
		t.Fatalf("AOS-CX switch: %+v", got[cx.ID])
	}
	if got[paused.ID].Editable || got[paused.ID].ReadOnly != "disabled" {
		t.Fatalf("disabled switch: %+v", got[paused.ID])
	}
	if got[web.ID].Editable || got[web.ID].ReadOnly == "" {
		t.Fatalf("web switch: %+v", got[web.ID])
	}
}

// Two branches joined only by routing reuse 192.168.13.0/24. A switch in the
// first branch sees a local device at 192.168.13.2, the same address as the
// managed switch in the other branch. That must not draw a link between them.
func TestTopologyOverlappingBranchIPs(t *testing.T) {
	fl2 := store.Switch{ID: "fl2", Host: "172.16.254.14"}
	digomi := store.Switch{ID: "digomi", Host: "192.168.13.2"}
	fl2Snap := &aruba.Snapshot{
		Facts: aruba.Facts{Hostname: "SW-HP-FL2-R", BaseMAC: "ec:02:73:00:00:01"},
		STP:   aruba.STP{Enabled: true, Ports: []aruba.STPPort{{Port: "24", State: "Blocking"}}},
		Neighbors: []aruba.Neighbor{
			// a local switch in FL2's branch that happens to use 192.168.13.2
			{Protocol: "lldp", LocalPort: "24", MgmtIP: "192.168.13.2", ChassisID: "c0:74:ad:ea:29:d8", Capabilities: "Bridge"},
		},
	}
	digomiSnap := &aruba.Snapshot{
		Facts: aruba.Facts{Hostname: "SW-DIGOMI-M", BaseMAC: "38:21:c7:00:00:02"},
		STP:   aruba.STP{Enabled: true, IsRoot: true},
	}
	topo := buildTopology([]switchInfo{{sw: fl2, snap: fl2Snap, online: true}, {sw: digomi, snap: digomiSnap, online: true}}, nil, false)
	for _, e := range topo.Edges {
		if (e.From == "fl2" && e.To == "digomi") || (e.From == "digomi" && e.To == "fl2") {
			t.Fatalf("drew a link across branches: %+v", e)
		}
	}
	if len(topo.Edges) != 1 || len(topo.Nodes) != 3 {
		t.Fatalf("want FL2 linked to its own neighbor: nodes %+v edges %+v", topo.Nodes, topo.Edges)
	}

	// A stack advertises a MAC we never read, but both sides report the link,
	// so it is still drawn as one link between the managed switches.
	stack := store.Switch{ID: "stack", Host: "10.1.1.1"}
	acc := store.Switch{ID: "acc", Host: "10.1.1.2"}
	stackSnap := &aruba.Snapshot{
		Facts:     aruba.Facts{Hostname: "CORE-STACK", BaseMAC: "00:00:00:00:00:aa"},
		Neighbors: []aruba.Neighbor{{Protocol: "lldp", LocalPort: "1/1/1", MgmtIP: "10.1.1.2", ChassisID: "00:00:00:00:00:bb", PortID: "49"}},
	}
	accSnap := &aruba.Snapshot{
		Facts:     aruba.Facts{Hostname: "ACC", BaseMAC: "00:00:00:00:00:bb"},
		Neighbors: []aruba.Neighbor{{Protocol: "lldp", LocalPort: "49", MgmtIP: "10.1.1.1", ChassisID: "00:00:00:00:99:99", PortID: "1/1/1"}},
	}
	topo = buildTopology([]switchInfo{{sw: stack, snap: stackSnap, online: true}, {sw: acc, snap: accSnap, online: true}}, nil, false)
	if len(topo.Nodes) != 2 || len(topo.Edges) != 1 {
		t.Fatalf("stack link: nodes %+v edges %+v", topo.Nodes, topo.Edges)
	}
}

func TestOpticIssues(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	snap := &aruba.Snapshot{
		Family: aruba.FamilyAOSCX, CollectedAt: time.Now(),
		Ports: []aruba.Port{{Name: "1/1/49", Up: true, Enabled: true}, {Name: "1/1/50", Up: true, Enabled: true}, {Name: "1/1/51", Up: false, Enabled: true}, {Name: "1/1/52", Up: true, Enabled: true}},
		Optics: []aruba.Optic{
			{Port: "1/1/49", Type: "SFP+LR", DOM: true, Rx: []float64{-14.2}, RxLowWarn: f(-14), RxLowAlarm: f(-18)},
			{Port: "1/1/50", Type: "SFP+SR", DOM: true, Rx: []float64{-19.5}, RxLowWarn: f(-10), RxLowAlarm: f(-13)},
			{Port: "1/1/51", Type: "SFP+SR", DOM: true, Rx: []float64{aruba.NoLight}, RxLowWarn: f(-10), RxLowAlarm: f(-13)}, // empty port
			{Port: "1/1/52", Type: "SFP+SR", DOM: true, Rx: []float64{-3}, RxLowWarn: f(-10), RxLowAlarm: f(-13)},
		},
	}
	is := evaluate(store.Switch{ID: "x", Host: "10.0.0.1"}, &Status{Online: true, Checked: true}, snap, store.DefaultSettings())
	var crit, warn *Issue
	for i := range is {
		switch is[i].Title {
		case "Optical power alarm on 1 port(s)":
			crit = &is[i]
		case "Optical power warning on 1 port(s)":
			warn = &is[i]
		}
	}
	if crit == nil || !strings.Contains(crit.Detail, "1/1/50") || strings.Contains(crit.Detail, "1/1/51") {
		t.Fatalf("alarm: %+v", is)
	}
	if warn == nil || !strings.Contains(warn.Detail, "1/1/49 (SFP+LR): Rx -14.2 dBm, warning at -14.0") {
		t.Fatalf("warning: %+v", warn)
	}
}

func TestFindMACAndNewRules(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, _ := st.SaveSwitch(store.Switch{Name: "acc", Host: "10.0.0.2", Port: 22})
	c, _ := st.SaveSwitch(store.Switch{Name: "core", Host: "10.0.0.1", Port: 22})
	m := New(st, nil)
	<-m.loaded
	accSnap := &aruba.Snapshot{Family: aruba.FamilyAOSS, Facts: aruba.Facts{Hostname: "acc", BaseMAC: "00:00:00:00:00:aa"},
		Ports:     []aruba.Port{{Name: "5", Description: "printer"}, {Name: "48"}},
		Neighbors: []aruba.Neighbor{{Protocol: "lldp", LocalPort: "48", SysName: "core", ChassisID: "00:00:00:00:00:cc", Capabilities: "Bridge"}},
		MACs:      []aruba.MACEntry{{MAC: "12:34:56:78:9a:bc", Port: "5", VLAN: "10"}, {MAC: "12:34:56:78:9a:bd", Port: "48", VLAN: "10"}}}
	coreSnap := &aruba.Snapshot{Family: aruba.FamilyAOSCX, Facts: aruba.Facts{Hostname: "core", BaseMAC: "00:00:00:00:00:cc"},
		Ports:     []aruba.Port{{Name: "1/1/1"}},
		Neighbors: []aruba.Neighbor{{Protocol: "lldp", LocalPort: "1/1/1", SysName: "acc", ChassisID: "00:00:00:00:00:aa", Capabilities: "Bridge"}},
		MACs:      []aruba.MACEntry{{MAC: "12:34:56:78:9a:bc", Port: "1/1/1", VLAN: "10"}}}
	m.mu.Lock()
	m.snaps[a.ID], m.snaps[c.ID] = accSnap, coreSnap
	m.status[a.ID], m.status[c.ID] = &Status{Online: true, Checked: true}, &Status{Online: true, Checked: true}
	m.mu.Unlock()
	hits := m.FindMAC("12:34:56:78:9A:BC")
	if len(hits) != 2 || !hits[0].Edge || hits[0].SwitchName != "acc" || hits[0].Port != "5" || hits[0].PortName != "printer" || hits[1].Edge || hits[1].Uplink != "acc" {
		t.Fatalf("hits = %+v", hits)
	}
	if h := m.FindMAC("9abd"); len(h) != 1 || h[0].Uplink != "core" {
		t.Fatalf("suffix search = %+v", h)
	}
	all := m.Clients()
	if len(all) != 2 || all[0].MAC != "12:34:56:78:9a:bc" || !all[0].Edge || all[0].Seen != 2 || all[0].SwitchName != "acc" || all[1].MAC != "12:34:56:78:9a:bd" {
		t.Fatalf("clients = %+v", all)
	}
	if !portLess("1/1/2", "1/1/10") || portLess("1/1/10", "1/1/2") || !portLess("5", "48") {
		t.Fatal("port order")
	}
	if m.FindMAC("12") != nil {
		t.Fatal("too short a query should give nothing")
	}

	snap := &aruba.Snapshot{Family: aruba.FamilyAOSS, Facts: aruba.Facts{Hostname: "s", CPU: -1, Memory: -1}, RunningConfig: "hostname s\n", CollectedAt: time.Now(),
		Security: aruba.Security{SSHCiphers: []string{"aes256-ctr", "3des-cbc"}, NTPServers: []string{"x"}, LogServers: []string{"x"}, AAA: []string{"x"}, ManagerPassword: true},
		Logs: []aruba.LogLine{{Text: "auth: Invalid user name/password on SSH session"}, {Text: "auth: Invalid user name/password on SSH session"}, {Text: "auth: Invalid user name/password on SSH session"},
			{Text: "ports: port 7 is now off-line"}, {Text: "ports: port 7 is now off-line"}, {Text: "ports: port 7 is now off-line"}},
		Stack: &aruba.Stack{Topology: "Chain", Members: []aruba.StackMember{{ID: "1", Status: "Commander", OK: true}, {ID: "2", Status: "Missing"}}}}
	was := localIsUTC
	defer func() { localIsUTC = was }()
	localIsUTC = func() bool { return false }
	s := &Status{Online: true, Checked: true, UsersAdded: []string{"eve (manager)"}, UsersChanged: time.Now()}
	want := map[string]bool{"Outdated SSH algorithms enabled": false, "No time zone set": false, "3 failed login attempts in the recent log": false, "1 port(s) flapping": false, "Local user accounts changed": false, "Stack member 2 is missing": false, "Stack ring is broken": false}
	for _, i := range evaluate(a, s, snap, store.DefaultSettings()) {
		if _, ok := want[i.Title]; ok {
			want[i.Title] = true
		}
	}
	for k, v := range want {
		if !v {
			t.Errorf("missing issue %q", k)
		}
	}
}

// Issue #24: a switch without a time zone runs on UTC, which is right in
// London, so there's nothing to note when this computer is on UTC/GMT.
func TestNoTimeZoneNoteOnUTC(t *testing.T) {
	was := localIsUTC
	defer func() { localIsUTC = was }()
	sw := store.Switch{ID: "x", Name: "x"}
	snap := &aruba.Snapshot{Family: aruba.FamilyAOSS, Facts: aruba.Facts{CPU: -1, Memory: -1}, RunningConfig: "hostname x\n", CollectedAt: time.Now()}
	st := &Status{Online: true, Checked: true}
	has := func() bool {
		for _, i := range evaluate(sw, st, snap, store.DefaultSettings()) {
			if i.Title == "No time zone set" {
				return true
			}
		}
		return false
	}
	localIsUTC = func() bool { return true }
	if has() {
		t.Fatal("noted on a computer in GMT")
	}
	localIsUTC = func() bool { return false }
	if !has() {
		t.Fatal("not noted on a computer outside UTC")
	}
}

func TestMulticastIGMPAudit(t *testing.T) {
	swA := store.Switch{ID: "a", Name: "core", Host: "10.0.0.1"}
	swB := store.Switch{ID: "b", Name: "access", Host: "10.0.0.2"}

	// Audio/DANTE VLAN with active ports and IGMP snooping disabled should warn per switch
	snapOff := &aruba.Snapshot{
		Family:      aruba.FamilyAOSCX,
		CollectedAt: time.Now(),
		Facts:       aruba.Facts{Hostname: "access", CPU: -1, Memory: -1},
		VLANs: []aruba.VLAN{
			{ID: 1, Name: "DEFAULT_VLAN_1", Untagged: []string{"1/1/2"}, IGMPConfigKnown: true, IGMPSnooping: false},
			{ID: 20, Name: "Audio-DANTE", Untagged: []string{"1/1/5"}, IGMPConfigKnown: true, IGMPSnooping: false},
		},
	}
	issues := evaluate(swB, &Status{Online: true, Checked: true}, snapOff, store.DefaultSettings())
	var mcastWarn *Issue
	for i := range issues {
		if issues[i].Category == "Multicast" {
			mcastWarn = &issues[i]
		}
	}
	if mcastWarn == nil || mcastWarn.Severity != Warning || !strings.Contains(mcastWarn.Title, "VLAN 20 (Audio-DANTE)") {
		t.Fatalf("expected multicast warning for VLAN 20, got: %+v", issues)
	}

	// Non-audio VLAN with DSCP-trusted port should also trigger multicast audit if IGMP snooping off
	snapDSCP := &aruba.Snapshot{
		Family:      aruba.FamilyAOSS,
		CollectedAt: time.Now(),
		Facts:       aruba.Facts{Hostname: "dscp", CPU: -1, Memory: -1},
		Ports:       []aruba.Port{{Name: "7", QoSTrust: "dscp"}},
		VLANs:       []aruba.VLAN{{ID: 88, Name: "StageNet", Untagged: []string{"7"}, IGMPConfigKnown: true, IGMPSnooping: false}},
	}
	issuesDSCP := evaluate(swB, &Status{Online: true, Checked: true}, snapDSCP, store.DefaultSettings())
	foundDSCP := false
	for _, is := range issuesDSCP {
		if is.Category == "Multicast" && strings.Contains(is.Title, "VLAN 88") {
			foundDSCP = true
		}
	}
	if !foundDSCP {
		t.Fatalf("expected DSCP-tagged VLAN to be audited for IGMP snooping: %+v", issuesDSCP)
	}

	// Network-wide L2 audit: IGMP snooping active with NO querier -> Info; with querier -> none
	snapsNoQ := map[string]*aruba.Snapshot{
		swA.ID: {VLANs: []aruba.VLAN{{ID: 20, Name: "Audio", Untagged: []string{"1/1/1"}, IGMPConfigKnown: true, IGMPSnooping: true, IGMPQuerier: false}}},
		swB.ID: {VLANs: []aruba.VLAN{{ID: 20, Name: "Audio", Untagged: []string{"1/1/5"}, IGMPConfigKnown: true, IGMPSnooping: true, IGMPQuerier: false}}},
	}
	topo := Topology{Edges: []Edge{{From: swA.ID, To: swB.ID}}}
	netNoQ := networkIssues(snapsNoQ, map[string]string{swA.ID: "core", swB.ID: "access"}, topo)
	if len(netNoQ) != 1 || netNoQ[0].Severity != Info || !strings.Contains(netNoQ[0].Title, "No IGMP querier") {
		t.Fatalf("expected no-querier info issue, got: %+v", netNoQ)
	}

	snapsWithQ := map[string]*aruba.Snapshot{
		swA.ID: {VLANs: []aruba.VLAN{{ID: 20, Name: "Audio", Untagged: []string{"1/1/1"}, IGMPConfigKnown: true, IGMPSnooping: true, IGMPQuerier: true}}},
		swB.ID: {VLANs: []aruba.VLAN{{ID: 20, Name: "Audio", Untagged: []string{"1/1/5"}, IGMPConfigKnown: true, IGMPSnooping: true, IGMPQuerier: false}}},
	}
	netWithQ := networkIssues(snapsWithQ, map[string]string{swA.ID: "core", swB.ID: "access"}, topo)
	if len(netWithQ) != 0 {
		t.Fatalf("expected no network issue when querier exists, got: %+v", netWithQ)
	}
}

func TestDanteQoSAudit(t *testing.T) {
	sw := store.Switch{ID: "cx1", Name: "Stage-CX", Host: "10.0.0.51"}
	snap := &aruba.Snapshot{
		Family:      aruba.FamilyAOSCX,
		CollectedAt: time.Now(),
		Facts:       aruba.Facts{Hostname: "Stage-CX", CPU: -1, Memory: -1},
		Ports: []aruba.Port{
			{Name: "1/1/5", Enabled: true, Up: true, EEEOn: true},
			{Name: "1/1/24", Enabled: true, Up: true},
		},
		VLANs: []aruba.VLAN{
			{ID: 20, Name: "Audio-DANTE", Untagged: []string{"1/1/5"}, Tagged: []string{"1/1/24"}, IGMPConfigKnown: true, IGMPSnooping: true},
		},
		QoS: &aruba.QoSInfo{Known: true, DSCP56Priority: -1, DSCP46Priority: -1},
	}
	issues := evaluate(sw, &Status{Online: true, Checked: true}, snap, store.DefaultSettings())
	var sawTrust, sawEEE, sawStrict bool
	for _, is := range issues {
		if is.Category != "Dante QoS" {
			continue
		}
		if strings.Contains(is.Title, "DSCP trust missing") {
			sawTrust = true
		}
		if strings.Contains(is.Title, "Energy-Efficient Ethernet") {
			sawEEE = true
		}
		if strings.Contains(is.Title, "Strict priority") {
			sawStrict = true
		}
	}
	if !sawTrust || !sawEEE || !sawStrict {
		t.Fatalf("expected trust+eee+strict Dante issues, got: %+v", issues)
	}

	// After applying Dante AV Profile, all Dante QoS issues should clear
	fixed := aruba.AfterDanteProfileChange(snap, aruba.DanteProfileChange{
		VLANIDs:        []int{20},
		Ports:          []string{"1/1/5", "1/1/24"},
		GlobalTrust:    true,
		DSCPMap:        true,
		StrictSchedule: true,
		IGMPSnooping:   true,
		DisableEEE:     true,
	})
	afterIssues := evaluate(sw, &Status{Online: true, Checked: true}, fixed, store.DefaultSettings())
	for _, is := range afterIssues {
		if is.Category == "Dante QoS" {
			t.Fatalf("expected zero Dante QoS issues after profile apply, got: %+v", is)
		}
	}
}

func TestSearchConfigs(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, _ := st.SaveSwitch(store.Switch{Name: "Core-A", Host: "10.0.0.1", Site: "Building-A"})
	b, _ := st.SaveSwitch(store.Switch{Name: "Stage-B", Host: "10.0.0.2", Site: "Building-B"})
	c, _ := st.SaveSwitch(store.Switch{Name: "Lab-C", Host: "10.0.0.3"})
	m := New(st, nil)
	<-m.loaded
	m.mu.Lock()
	m.snaps[a.ID] = &aruba.Snapshot{Family: aruba.FamilyAOSCX, RunningConfig: "hostname Core-A\nvlan 20\n    name Audio\n    ip igmp snooping enable\nqos trust dscp\n"}
	m.snaps[b.ID] = &aruba.Snapshot{Family: aruba.FamilyAOSS, RunningConfig: "hostname Stage-B\nvlan 20\n   name \"DANTE\"\n   ip igmp\n"}
	m.snaps[c.ID] = &aruba.Snapshot{Family: aruba.FamilyAOSCX, RunningConfig: "hostname Lab-C\nvlan 10\n    name Office\n"}
	m.mu.Unlock()

	hits, err := m.SearchConfigs("vlan 20", false, "")
	if err != nil || len(hits) != 2 {
		t.Fatalf("SearchConfigs plain: err=%v hits=%+v", err, hits)
	}
	if hits[0].SwitchName != "Core-A" || hits[0].Lines[0].Line != 2 {
		t.Fatalf("unexpected first hit: %+v", hits[0])
	}

	siteHits, err := m.SearchConfigs("igmp", false, "Building-A")
	if err != nil || len(siteHits) != 1 || siteHits[0].SwitchID != a.ID {
		t.Fatalf("site filter failed: %+v", siteHits)
	}

	noneHits, err := m.SearchConfigs("Office", false, "__none__")
	if err != nil || len(noneHits) != 1 || noneHits[0].SwitchID != c.ID {
		t.Fatalf("__none__ site filter failed: %+v", noneHits)
	}

	reHits, err := m.SearchConfigs(`name\s+"?dante`, true, "")
	if err != nil || len(reHits) != 1 || reHits[0].SwitchID != b.ID {
		t.Fatalf("regex search failed: err=%v hits=%+v", err, reHits)
	}
	if _, err := m.SearchConfigs("[unclosed", true, ""); err == nil {
		t.Fatal("expected invalid regex error")
	}
}

func TestPhaseBAudits(t *testing.T) {
	swA := store.Switch{ID: "a", Name: "Core-A", Host: "10.0.0.1"}
	swB := store.Switch{ID: "b", Name: "Stage-B", Host: "10.0.0.2"}

	// Power budget high (84%), half-duplex port, AP uplink downgraded to 100M, access port at 100M
	snap := &aruba.Snapshot{
		Family:      aruba.FamilyAOSCX,
		CollectedAt: time.Now(),
		Facts:       aruba.Facts{Hostname: "Core-A", CPU: -1, Memory: -1, PowerUsedW: 620, PowerMaxW: 740},
		Neighbors:   []aruba.Neighbor{{Protocol: "lldp", LocalPort: "1/1/10", SysName: "AP-Stage", Capabilities: "WLAN Access Point"}},
		Ports: []aruba.Port{
			{Name: "1/1/10", Enabled: true, Up: true, Speed: "100FDx", Type: "100/1000T", Description: "AP-Stage"},
			{Name: "1/1/11", Enabled: true, Up: true, Speed: "100HDx", Type: "100/1000T"},
			{Name: "1/1/12", Enabled: true, Up: true, Speed: "100FDx", Type: "100/1000T", Description: "Printer"},
			{Name: "1/1/13", Enabled: true, Up: true, Speed: "1000FDx", Type: "100/1000T"},
		},
	}
	issues := evaluate(swA, &Status{Online: true, Checked: true}, snap, store.DefaultSettings())
	var sawPower, sawHalf, sawInfra, sawAccess bool
	for _, is := range issues {
		if is.Category == "Power" && is.Severity == Warning && strings.Contains(is.Title, "84%") {
			sawPower = true
		}
		if is.Category == "Ports" && is.Severity == Warning && strings.Contains(is.Title, "half-duplex") {
			sawHalf = true
		}
		if is.Category == "Ports" && is.Severity == Warning && strings.Contains(is.Title, "uplink/AP") {
			sawInfra = true
		}
		if is.Category == "Ports" && is.Severity == Info && strings.Contains(is.Title, "100M/10M") {
			sawAccess = true
		}
	}
	if !sawPower || !sawHalf || !sawInfra || !sawAccess {
		t.Fatalf("missing expected Phase B issues: power=%v half=%v infra=%v access=%v; got=%+v", sawPower, sawHalf, sawInfra, sawAccess, issues)
	}

	// Trunk VLAN mismatch across switch-to-switch edge (handling LAG alias Trk1 too!)
	sCore := &aruba.Snapshot{
		Ports: []aruba.Port{{Name: "24", Trunk: "Trk1"}},
		VLANs: []aruba.VLAN{{ID: 1, Untagged: []string{"Trk1"}}, {ID: 20, Name: "Audio", Tagged: []string{"Trk1"}}},
	}
	sStage := &aruba.Snapshot{
		Ports: []aruba.Port{{Name: "1/1/24"}},
		VLANs: []aruba.VLAN{{ID: 1, Untagged: []string{"1/1/24"}}, {ID: 20, Name: "Audio", Untagged: []string{"1/1/5"}}}, // forgot 1/1/24!
	}
	topo := Topology{Edges: []Edge{{From: swA.ID, To: swB.ID, FromPort: "24", ToPort: "1/1/24"}}}
	netIssues := networkIssues(map[string]*aruba.Snapshot{swA.ID: sCore, swB.ID: sStage}, map[string]string{swA.ID: "Core-A", swB.ID: "Stage-B"}, topo)
	if len(netIssues) != 1 || netIssues[0].Severity != Warning || !strings.Contains(netIssues[0].Title, "Trunk VLAN mismatch") || !strings.Contains(netIssues[0].Detail, "VLAN 20") {
		t.Fatalf("expected trunk VLAN mismatch warning, got: %+v", netIssues)
	}

	// Once Stage-B tags 1/1/24 on VLAN 20, mismatch clears
	sStage.VLANs[1].Tagged = []string{"1/1/24"}
	netClean := networkIssues(map[string]*aruba.Snapshot{swA.ID: sCore, swB.ID: sStage}, map[string]string{swA.ID: "Core-A", swB.ID: "Stage-B"}, topo)
	if len(netClean) != 0 {
		t.Fatalf("expected zero trunk mismatches after tagging both ends, got: %+v", netClean)
	}
}

func TestTopologyVLANOverlayAndVendor(t *testing.T) {
	swA := store.Switch{ID: "sw-a", Name: "Core-A", Host: "10.0.0.1"}
	swB := store.Switch{ID: "sw-b", Name: "Stage-B", Host: "10.0.0.2"}

	snapA := &aruba.Snapshot{
		Facts: aruba.Facts{Hostname: "Core-A", BaseMAC: "00:0b:86:01:02:03"},
		Ports: []aruba.Port{
			{Name: "24", Up: true, Trunk: "Trk1"},
			{Name: "5", Up: true, VLAN: "20"},
		},
		VLANs: []aruba.VLAN{
			{ID: 1, Name: "DEFAULT_VLAN", Untagged: []string{"Trk1"}},
			{ID: 20, Name: "Audio", Tagged: []string{"Trk1"}, Untagged: []string{"5"}},
			{ID: 30, Name: "Video", Tagged: []string{"Trk1"}},
		},
		Neighbors: []aruba.Neighbor{
			{Protocol: "lldp", LocalPort: "24", SysName: "Stage-B", ChassisID: "24:de:c6:aa:bb:cc", PortID: "1/1/24", MgmtIP: "10.0.0.2"},
			{Protocol: "lldp", LocalPort: "5", SysName: "00:1d:c1:11:22:33", ChassisID: "00:1d:c1:11:22:33", PortID: "eth0"},
		},
	}
	snapB := &aruba.Snapshot{
		Facts: aruba.Facts{Hostname: "Stage-B", BaseMAC: "24:de:c6:aa:bb:cc"},
		Ports: []aruba.Port{
			{Name: "1/1/24", Up: true},
		},
		VLANs: []aruba.VLAN{
			{ID: 1, Name: "DEFAULT_VLAN", Untagged: []string{"1/1/24"}},
			{ID: 20, Name: "Audio", Tagged: []string{"1/1/24"}},
			{ID: 30, Name: "Video"}, // forgot to tag 1/1/24 on VLAN 30!
		},
		Neighbors: []aruba.Neighbor{
			{Protocol: "lldp", LocalPort: "1/1/24", SysName: "Core-A", ChassisID: "00:0b:86:01:02:03", PortID: "24", MgmtIP: "10.0.0.1"},
		},
	}

	topo := buildTopology([]switchInfo{
		{sw: swA, snap: snapA, online: true},
		{sw: swB, snap: snapB, online: true},
	}, nil, true)

	if len(topo.VLANs) != 3 || topo.VLANs[1].ID != 20 || topo.VLANs[1].Name != "Audio" || topo.VLANs[1].Count != 2 {
		t.Fatalf("unexpected topology VLAN list: %+v", topo.VLANs)
	}

	var leaf *Node
	for i := range topo.Nodes {
		if !topo.Nodes[i].Managed {
			leaf = &topo.Nodes[i]
		}
	}
	if leaf == nil || leaf.Vendor != "Audinate (Dante)" || leaf.MAC != "00:1d:c1:11:22:33" {
		t.Fatalf("expected unmanaged leaf node with Audinate (Dante) vendor, got: %+v", leaf)
	}

	var trunk *Edge
	for i := range topo.Edges {
		e := &topo.Edges[i]
		if (e.From == "sw-a" && e.To == "sw-b") || (e.From == "sw-b" && e.To == "sw-a") {
			trunk = e
		}
	}
	if trunk == nil {
		t.Fatalf("expected inter-switch trunk edge, got: %+v", topo.Edges)
	}
	// sw-a < sw-b alphabetically, so From=sw-a (carries 1,20,30 via Trk1), To=sw-b (carries 1,20 only)
	if len(trunk.FromVLANs) != 3 || trunk.FromVLANs[2] != 30 {
		t.Fatalf("expected FromVLANs=[1 20 30] via Trk1 alias, got: %+v", trunk.FromVLANs)
	}
	if len(trunk.ToVLANs) != 2 || trunk.ToVLANs[1] != 20 {
		t.Fatalf("expected ToVLANs=[1 20] (missing 30), got: %+v", trunk.ToVLANs)
	}
}

func TestClientsAndDetailVendorBackfill(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sw, err := st.SaveSwitch(store.Switch{Name: "Stage-A", Host: "10.0.0.10"})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate an older on-disk snapshot that lacks Vendor field on MAC entries
	snap := &aruba.Snapshot{
		CollectedAt: time.Now(),
		Ports:       []aruba.Port{{Name: "1", Enabled: true, Up: true, Description: "Console"}},
		MACs: []aruba.MACEntry{
			{MAC: "00:a0:de:11:22:33", Port: "1", VLAN: "20"},
			{MAC: "00:0e:dd:44:55:66", Port: "1", VLAN: "20"},
		},
	}
	if err := st.SaveSnapshot(sw.ID, *snap); err != nil {
		t.Fatal(err)
	}
	m := New(st, nil)
	<-m.loaded
	clients := m.Clients()
	if len(clients) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(clients))
	}
	gotV := map[string]string{}
	for _, c := range clients {
		gotV[c.MAC] = c.Vendor
	}
	if gotV["00:a0:de:11:22:33"] != "Yamaha" || gotV["00:0e:dd:44:55:66"] != "Shure" {
		t.Fatalf("expected Yamaha and Shure vendors from Clients(), got: %+v", gotV)
	}
	det, err := m.Detail(sw.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(det.Snapshot.MACs) != 2 || det.Snapshot.MACs[0].Vendor != "Yamaha" || det.Snapshot.MACs[1].Vendor != "Shure" {
		t.Fatalf("expected Detail() to backfill Vendor on cached snapshot MACs, got: %+v", det.Snapshot.MACs)
	}
}
