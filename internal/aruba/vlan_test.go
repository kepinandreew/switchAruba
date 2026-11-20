package aruba

import (
	"reflect"
	"testing"
)

func TestAfterChange(t *testing.T) {
	cur := []VLAN{
		{ID: 1, Name: "DEFAULT", Untagged: []string{"1", "2", "3"}},
		{ID: 20, Name: "Users", Untagged: []string{"4"}, Tagged: []string{"24"}},
	}
	ids := func(vs []VLAN) []int {
		var out []int
		for _, v := range vs {
			out = append(out, v.ID)
		}
		return out
	}

	if got := ids(AfterChange(cur, VLANChange{Action: "delete", ID: 20})); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("delete: %v", got)
	}

	got := AfterChange(cur, VLANChange{Action: "create", ID: 10, Name: "Voice", AddUntagged: []string{"2"}, AddTagged: []string{"24"}})
	if !reflect.DeepEqual(ids(got), []int{1, 10, 20}) {
		t.Fatalf("create order: %v", ids(got))
	}
	if !reflect.DeepEqual(got[0].Untagged, []string{"1", "3"}) {
		t.Fatalf("port 2 should leave VLAN 1 untagged: %v", got[0].Untagged)
	}
	if got[1].Name != "Voice" || !reflect.DeepEqual(got[1].Tagged, []string{"24"}) {
		t.Fatalf("new VLAN: %+v", got[1])
	}

	got = AfterChange(cur, VLANChange{Action: "update", ID: 20, Name: "Staff", RemovePorts: []string{"24"}})
	if got[1].Name != "Staff" || len(got[1].Tagged) != 0 || !reflect.DeepEqual(got[1].Untagged, []string{"4"}) {
		t.Fatalf("update: %+v", got[1])
	}

	if cur[0].Name != "DEFAULT" || len(cur[0].Untagged) != 3 || len(cur[1].Tagged) != 1 {
		t.Fatalf("the current table must not be changed: %+v", cur)
	}

	got = AfterChange(cur, VLANChange{Action: "update", ID: 20, AddUntagged: []string{"4"}, AddTagged: []string{"24"}})
	if !reflect.DeepEqual(got[1].Untagged, []string{"4"}) || !reflect.DeepEqual(got[1].Tagged, []string{"24"}) {
		t.Fatalf("ports added again are listed twice: %+v", got[1])
	}
}

func TestBuildPortVLANCommands(t *testing.T) {
	cur := []VLAN{{ID: 1, Name: "DEFAULT"}}

	// Test AOS-CX Access
	cxCmds, err := BuildPortVLANCommands(FamilyAOSCX, PortVLANChange{
		Ports:      []string{"1/1/1", "1/1/2"},
		Mode:       "access",
		AccessVLAN: 30,
		Save:       true,
	}, cur)
	if err != nil {
		t.Fatalf("cx error: %v", err)
	}
	expectedCX := []string{
		"configure terminal",
		"vlan 30", "exit",
		"interface 1/1/1", "vlan access 30", "exit",
		"interface 1/1/2", "vlan access 30", "exit",
		"end", "write memory",
	}
	if !reflect.DeepEqual(cxCmds, expectedCX) {
		t.Fatalf("expected CX cmds %+v, got %+v", expectedCX, cxCmds)
	}

	// Test AOS-S Access
	sCmds, err := BuildPortVLANCommands(FamilyAOSS, PortVLANChange{
		Ports:      []string{"1", "2"},
		Mode:       "access",
		AccessVLAN: 30,
		Save:       true,
	}, cur)
	if err != nil {
		t.Fatalf("aoss error: %v", err)
	}
	expectedS := []string{
		"configure terminal",
		"vlan 30", "exit",
		"vlan 30", "untagged 1-2", "exit",
		"end", "write memory",
	}
	if !reflect.DeepEqual(sCmds, expectedS) {
		t.Fatalf("expected S cmds %+v, got %+v", expectedS, sCmds)
	}

	// Test AOS-CX Trunk
	cxTrunk, err := BuildPortVLANCommands(FamilyAOSCX, PortVLANChange{
		Ports:       []string{"1/1/24"},
		Mode:        "trunk",
		AccessVLAN:  1,
		TaggedVLANs: []int{10, 20},
		Save:        false,
	}, cur)
	if err != nil {
		t.Fatalf("cx trunk error: %v", err)
	}
	expectedCXTrunk := []string{
		"configure terminal",
		"vlan 10", "exit",
		"vlan 20", "exit",
		"interface 1/1/24", "vlan trunk native 1", "vlan trunk allowed 1,10,20", "exit",
		"end",
	}
	if !reflect.DeepEqual(cxTrunk, expectedCXTrunk) {
		t.Fatalf("expected CX trunk %+v, got %+v", expectedCXTrunk, cxTrunk)
	}
}

func TestBuildPortPoECommands(t *testing.T) {
	cxOff, err := BuildPortPoECommands(FamilyAOSCX, PortPoEChange{Ports: []string{"1/1/2"}, Action: "off", Save: false})
	if err != nil {
		t.Fatal(err)
	}
	wantCXOff := []string{"configure terminal", "interface 1/1/2", "no power-over-ethernet", "exit", "end"}
	if !reflect.DeepEqual(cxOff, wantCXOff) {
		t.Fatalf("got %v, want %v", cxOff, wantCXOff)
	}

	sBounce, err := BuildPortPoECommands(FamilyAOSS, PortPoEChange{Ports: []string{"1", "2"}, Action: "bounce", Save: true})
	if err != nil {
		t.Fatal(err)
	}
	wantSBounce := []string{"configure terminal", "interface 1-2", "no power-over-ethernet", "power-over-ethernet", "exit", "end", "write memory"}
	if !reflect.DeepEqual(sBounce, wantSBounce) {
		t.Fatalf("got %v, want %v", sBounce, wantSBounce)
	}

	offMap := PortPoEDisabled("interface 1/1/2\n   no power-over-ethernet\ninterface 1/1/3\n   power-over-ethernet\n")
	if !offMap["1/1/2"] || offMap["1/1/3"] {
		t.Fatalf("unexpected PoE disabled map: %v", offMap)
	}
}

func TestPortVLANQoSTrust(t *testing.T) {
	cur := []VLAN{{ID: 1, Name: "DEFAULT"}, {ID: 20, Name: "AUDIO"}}

	cxCmds, err := BuildPortVLANCommands(FamilyAOSCX, PortVLANChange{
		Ports:      []string{"1/1/5"},
		Mode:       "access",
		AccessVLAN: 20,
		QoSTrust:   "dscp",
		Save:       false,
	}, cur)
	if err != nil {
		t.Fatalf("cx qos dscp error: %v", err)
	}
	wantCX := []string{"configure terminal", "interface 1/1/5", "vlan access 20", "qos trust dscp", "exit", "end"}
	if !reflect.DeepEqual(cxCmds, wantCX) {
		t.Fatalf("got %v, want %v", cxCmds, wantCX)
	}

	cxReset, err := BuildPortVLANCommands(FamilyAOSCX, PortVLANChange{
		Ports:      []string{"1/1/5"},
		Mode:       "access",
		AccessVLAN: 20,
		QoSTrust:   "default",
	}, cur)
	if err != nil {
		t.Fatal(err)
	}
	wantReset := []string{"configure terminal", "interface 1/1/5", "vlan access 20", "no qos trust", "exit", "end"}
	if !reflect.DeepEqual(cxReset, wantReset) {
		t.Fatalf("got %v, want %v", cxReset, wantReset)
	}

	sCmds, err := BuildPortVLANCommands(FamilyAOSS, PortVLANChange{
		Ports:      []string{"5"},
		Mode:       "access",
		AccessVLAN: 20,
		QoSTrust:   "dscp",
		Save:       false,
	}, cur)
	if err != nil {
		t.Fatalf("aoss qos dscp error: %v", err)
	}
	wantS := []string{"configure terminal", "vlan 20", "untagged 5", "exit", "qos type-of-service diff-services", "end"}
	if !reflect.DeepEqual(sCmds, wantS) {
		t.Fatalf("got %v, want %v", sCmds, wantS)
	}

	if _, err := BuildPortVLANCommands(FamilyAOSS, PortVLANChange{
		Ports:      []string{"5"},
		Mode:       "access",
		AccessVLAN: 20,
		QoSTrust:   "cos",
	}, cur); err == nil {
		t.Fatal("expected error for aos-s qos trust cos")
	}

	qosMap := PortQoSTrustCX("interface 1/1/5\n    vlan access 20\n    qos trust dscp\ninterface 1/1/6\n    qos trust dscp\n    no qos trust\n")
	if qosMap["1/1/5"] != "dscp" || qosMap["1/1/6"] != "" {
		t.Fatalf("unexpected qos map: %v", qosMap)
	}

	updated := AfterPortVLANPorts([]Port{{Name: "1/1/5"}, {Name: "1/1/6", QoSTrust: "dscp"}}, FamilyAOSCX, PortVLANChange{Ports: []string{"1/1/5"}, QoSTrust: "dscp"})
	if updated[0].QoSTrust != "dscp" || updated[1].QoSTrust != "dscp" {
		t.Fatalf("unexpected updated ports: %+v", updated)
	}
	cleared := AfterPortVLANPorts(updated, FamilyAOSCX, PortVLANChange{Ports: []string{"1/1/5"}, QoSTrust: "default"})
	if cleared[0].QoSTrust != "" || cleared[1].QoSTrust != "dscp" {
		t.Fatalf("unexpected cleared ports: %+v", cleared)
	}
}

func TestVLANIGMPParseBuildAndAfterChange(t *testing.T) {
	bPtr := func(b bool) *bool { return &b }

	// AOS-CX config parsing: VLAN 20 has snooping + SVI querier; VLAN 30 has neither.
	cxCfg := `vlan 20
    name AUDIO
    ip igmp snooping enable
vlan 30
    name GUEST
interface vlan 20
    ip address 10.0.20.1/24
    ip igmp enable
    ip igmp querier
`
	cxVLANs, _, _ := ParseConfigCX(cxCfg)
	if len(cxVLANs) != 2 {
		t.Fatalf("expected 2 CX VLANs, got %+v", cxVLANs)
	}
	if !cxVLANs[0].IGMPConfigKnown || !cxVLANs[0].IGMPSnooping || !cxVLANs[0].IGMPQuerier {
		t.Fatalf("expected VLAN 20 snooping+querier, got %+v", cxVLANs[0])
	}
	if !cxVLANs[1].IGMPConfigKnown || cxVLANs[1].IGMPSnooping || cxVLANs[1].IGMPQuerier {
		t.Fatalf("expected VLAN 30 no igmp, got %+v", cxVLANs[1])
	}

	// AOS-S config parsing: ip igmp with explicit no ip igmp querier vs ip igmp querier.
	sCfg := `vlan 20
   name "AUDIO"
   ip igmp
   no ip igmp querier
   exit
vlan 21
   name "DANTE"
   ip igmp
   ip igmp querier
   exit
vlan 10
   name "DATA"
   exit
`
	sVLANs, _, _ := ParseConfigAOSS(sCfg)
	byID := map[int]VLAN{}
	for _, v := range sVLANs {
		byID[v.ID] = v
	}
	if !byID[20].IGMPConfigKnown || !byID[20].IGMPSnooping || byID[20].IGMPQuerier {
		t.Fatalf("expected AOS-S VLAN 20 snooping only, got %+v", byID[20])
	}
	if !byID[21].IGMPConfigKnown || !byID[21].IGMPSnooping || !byID[21].IGMPQuerier {
		t.Fatalf("expected AOS-S VLAN 21 snooping+querier, got %+v", byID[21])
	}
	if !byID[10].IGMPConfigKnown || byID[10].IGMPSnooping {
		t.Fatalf("expected AOS-S VLAN 10 snooping off, got %+v", byID[10])
	}

	// BuildVLANCommands AOS-CX snooping + querier
	cxCmds, err := BuildVLANCommands(FamilyAOSCX, VLANChange{
		Action:       "update",
		ID:           20,
		IGMPSnooping: bPtr(true),
		IGMPQuerier:  bPtr(true),
		Save:         false,
	}, nil)
	if err != nil {
		t.Fatalf("cx build error: %v", err)
	}
	wantCX := []string{
		"configure terminal",
		"vlan 20", "ip igmp snooping enable", "exit",
		"interface vlan 20", "ip igmp enable", "ip igmp querier", "exit",
		"end",
	}
	if !reflect.DeepEqual(cxCmds, wantCX) {
		t.Fatalf("cx IGMP cmds = %v, want %v", cxCmds, wantCX)
	}

	// BuildVLANCommands AOS-S snooping only (explicit false querier)
	sCmds, err := BuildVLANCommands(FamilyAOSS, VLANChange{
		Action:       "update",
		ID:           20,
		IGMPSnooping: bPtr(true),
		IGMPQuerier:  bPtr(false),
		Save:         true,
	}, nil)
	if err != nil {
		t.Fatalf("aoss build error: %v", err)
	}
	wantS := []string{
		"configure terminal",
		"vlan 20", "ip igmp", "no ip igmp querier", "exit",
		"end", "write memory",
	}
	if !reflect.DeepEqual(sCmds, wantS) {
		t.Fatalf("aoss IGMP cmds = %v, want %v", sCmds, wantS)
	}

	// AfterChange preserves IGMP updates in memory
	after := AfterChange([]VLAN{{ID: 20, Name: "AUDIO", Untagged: []string{"1/1/1"}}}, VLANChange{
		Action:       "update",
		ID:           20,
		IGMPSnooping: bPtr(true),
		IGMPQuerier:  bPtr(true),
	})
	if len(after) != 1 || !after[0].IGMPConfigKnown || !after[0].IGMPSnooping || !after[0].IGMPQuerier {
		t.Fatalf("AfterChange did not set IGMP fields: %+v", after)
	}
}

func TestBuildPortAdminCommandsAndAfterChange(t *testing.T) {
	cxShut, err := BuildPortAdminCommands(FamilyAOSCX, PortAdminChange{Ports: []string{"1/1/3"}, Action: "disable", Save: false})
	if err != nil {
		t.Fatal(err)
	}
	wantCXShut := []string{"configure terminal", "interface 1/1/3", "shutdown", "exit", "end"}
	if !reflect.DeepEqual(cxShut, wantCXShut) {
		t.Fatalf("cx disable = %v, want %v", cxShut, wantCXShut)
	}

	cxBounce, err := BuildPortAdminCommands(FamilyAOSCX, PortAdminChange{Ports: []string{"1/1/3"}, Action: "bounce", Save: true})
	if err != nil {
		t.Fatal(err)
	}
	wantCXBounce := []string{"configure terminal", "interface 1/1/3", "shutdown", "no shutdown", "exit", "end", "write memory"}
	if !reflect.DeepEqual(cxBounce, wantCXBounce) {
		t.Fatalf("cx bounce = %v, want %v", cxBounce, wantCXBounce)
	}

	sBounce, err := BuildPortAdminCommands(FamilyAOSS, PortAdminChange{Ports: []string{"5", "6"}, Action: "bounce", Save: false})
	if err != nil {
		t.Fatal(err)
	}
	wantSBounce := []string{"configure terminal", "interface 5-6", "disable", "enable", "exit", "end"}
	if !reflect.DeepEqual(sBounce, wantSBounce) {
		t.Fatalf("aoss bounce = %v, want %v", sBounce, wantSBounce)
	}

	if _, err := BuildPortAdminCommands(FamilyInstantOn, PortAdminChange{Ports: []string{"1"}, Action: "disable"}); err == nil {
		t.Fatal("expected error for Instant On")
	}

	after := AfterPortAdminChange([]Port{{Name: "1/1/3", Enabled: true, Up: true}, {Name: "1/1/4", Enabled: true, Up: true}}, PortAdminChange{Ports: []string{"1/1/3"}, Action: "disable"})
	if after[0].Enabled || after[0].Up || !after[1].Enabled {
		t.Fatalf("AfterPortAdminChange disable = %+v", after)
	}
	reEnabled := AfterPortAdminChange(after, PortAdminChange{Ports: []string{"1/1/3"}, Action: "enable"})
	if !reEnabled[0].Enabled {
		t.Fatalf("AfterPortAdminChange enable = %+v", reEnabled)
	}
}

func TestDanteProfileAndQoSConfigParse(t *testing.T) {
	bPtr := func(b bool) *bool { return &b }

	// PortVLANChange DisableEEE
	cxEEE, err := BuildPortVLANCommands(FamilyAOSCX, PortVLANChange{
		Ports:      []string{"1/1/5"},
		Mode:       "access",
		AccessVLAN: 20,
		QoSTrust:   "dscp",
		DisableEEE: bPtr(true),
	}, []VLAN{{ID: 20, Name: "Audio"}})
	if err != nil {
		t.Fatal(err)
	}
	wantCXEEE := []string{"configure terminal", "interface 1/1/5", "vlan access 20", "qos trust dscp", "no energy-efficient-ethernet", "exit", "end"}
	if !reflect.DeepEqual(cxEEE, wantCXEEE) {
		t.Fatalf("cx port vlan EEE = %v, want %v", cxEEE, wantCXEEE)
	}

	// ParseQoSConfig AOS-CX
	cxCfg := `qos trust dscp
qos dscp-map 56 local-priority 7 name Dante_PTP
qos dscp-map 46 local-priority 6 name Dante_Audio
qos schedule-profile dante_strict
    strict queue 0
    strict queue 7
apply qos queue-profile factory-default schedule-profile dante_strict
interface 1/1/5
    no energy-efficient-ethernet
interface 1/1/6
    energy-efficient-ethernet
`
	qosCX, offCX, onCX := ParseQoSConfig(FamilyAOSCX, cxCfg)
	if qosCX == nil || !qosCX.Known || qosCX.GlobalTrust != "dscp" || qosCX.DSCP56Priority != 7 || qosCX.DSCP46Priority != 6 || qosCX.ScheduleProfile != "dante_strict" || !qosCX.StrictTopQueues {
		t.Fatalf("ParseQoSConfig CX = %+v", qosCX)
	}
	if !offCX["1/1/5"] || !onCX["1/1/6"] {
		t.Fatalf("ParseQoSConfig CX EEE off=%v on=%v", offCX, onCX)
	}

	// ParseQoSConfig AOS-S
	sCfg := `qos type-of-service diff-services
qos dscp-map 111000 priority 7
qos dscp-map 101110 priority 6
no interface 1-2 energy-efficient-ethernet
`
	qosS, offS, _ := ParseQoSConfig(FamilyAOSS, sCfg)
	if qosS == nil || !qosS.Known || qosS.GlobalTrust != "dscp" || qosS.DSCP56Priority != 7 || qosS.DSCP46Priority != 6 || !qosS.StrictTopQueues {
		t.Fatalf("ParseQoSConfig AOS-S = %+v", qosS)
	}
	if !offS["1"] || !offS["2"] {
		t.Fatalf("ParseQoSConfig AOS-S EEE off=%v", offS)
	}

	// BuildDanteProfileCommands AOS-CX
	cmdsCX, err := BuildDanteProfileCommands(FamilyAOSCX, DanteProfileChange{
		VLANIDs:        []int{20},
		Ports:          []string{"1/1/5"},
		GlobalTrust:    true,
		DSCPMap:        true,
		StrictSchedule: true,
		IGMPSnooping:   true,
		DisableEEE:     true,
		Save:           true,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantDanteCX := []string{
		"configure terminal",
		"qos trust dscp",
		"qos dscp-map 56 local-priority 7 name Dante_PTP",
		"qos dscp-map 46 local-priority 6 name Dante_Audio",
		"qos schedule-profile dante_strict",
		"strict queue 0", "strict queue 1", "strict queue 2", "strict queue 3",
		"strict queue 4", "strict queue 5", "strict queue 6", "strict queue 7",
		"exit",
		"apply qos queue-profile factory-default schedule-profile dante_strict",
		"vlan 20", "ip igmp snooping enable", "exit",
		"interface 1/1/5", "no energy-efficient-ethernet", "exit",
		"end", "write memory",
	}
	if !reflect.DeepEqual(cmdsCX, wantDanteCX) {
		t.Fatalf("Dante CX cmds = %v, want %v", cmdsCX, wantDanteCX)
	}

	// BuildDanteProfileCommands AOS-S
	cmdsS, err := BuildDanteProfileCommands(FamilyAOSS, DanteProfileChange{
		VLANIDs:      []int{20},
		Ports:        []string{"1", "2"},
		GlobalTrust:  true,
		DSCPMap:      true,
		IGMPSnooping: true,
		DisableEEE:   true,
		Save:         false,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantDanteS := []string{
		"configure terminal",
		"qos type-of-service diff-services",
		"qos dscp-map 111000 priority 7",
		"qos dscp-map 101110 priority 6",
		"vlan 20", "ip igmp", "exit",
		"interface 1-2", "no energy-efficient-ethernet", "exit",
		"end",
	}
	if !reflect.DeepEqual(cmdsS, wantDanteS) {
		t.Fatalf("Dante AOS-S cmds = %v, want %v", cmdsS, wantDanteS)
	}
}
