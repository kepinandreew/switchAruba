package aruba

import (
	"strings"
	"testing"
)

func TestExtractAndBuildTemplateWithoutIP(t *testing.T) {
	rawCX := `hostname Golden-Core
ntp enable
ntp server 10.0.0.254
qos trust dscp
qos dscp-map 56 local-priority 7 name Dante_PTP
qos dscp-map 46 local-priority 6 name Dante_Audio
vlan 1
vlan 20
    name Audio_Dante
    ip igmp snooping enable
vlan 30
    name Video_NDI
interface vlan 1
    ip address 10.0.0.99/24
interface 1/1/1
    description StageBox-1
    vlan access 20
    qos trust dscp
    no energy-efficient-ethernet
    mtu 9198
interface 1/1/24
    description Uplink-Core
    vlan trunk native 1
    vlan trunk allowed 1,20,30
`
	tpl, err := ExtractTemplateFromConfigText("Standard Stage 24P", "Dante + Video template", rawCX)
	if err != nil {
		t.Fatalf("ExtractTemplateFromConfigText failed: %v", err)
	}
	if !tpl.DanteQoS || !tpl.JumboMTU || len(tpl.VLANs) != 3 || len(tpl.Ports) != 2 {
		t.Fatalf("unexpected extracted template: dante=%v jumbo=%v vlans=%d ports=%d", tpl.DanteQoS, tpl.JumboMTU, len(tpl.VLANs), len(tpl.Ports))
	}

	// 1. Compile for an AOS-CX target switch (and skip port 1/1/24 uplink)
	targetCX := &Snapshot{
		Family: FamilyAOSCX,
		Ports:  []Port{{Name: "1/1/1"}, {Name: "1/1/2"}, {Name: "1/1/24"}},
	}
	cmdsCX, err := BuildTemplateCommands(FamilyAOSCX, targetCX, tpl, TemplateApplyRequest{
		NewHostname:      "SW-FOH-NEW",
		IncludeVLANs:     true,
		IncludeIGMP:      true,
		IncludeDanteQoS:  true,
		JumboMTU:         true,
		IncludePortVLANs: true,
		IncludePortNames: true,
		IncludeSystem:    true,
		SkipPorts:        []string{"1/1/24"},
		Save:             true,
	})
	if err != nil {
		t.Fatalf("BuildTemplateCommands CX failed: %v", err)
	}
	joinedCX := strings.Join(cmdsCX, "\n")
	if strings.Contains(joinedCX, "10.0.0.99") || strings.Contains(joinedCX, "ip address") || strings.Contains(joinedCX, "Golden-Core") {
		t.Fatalf("template leaked source IP or source hostname:\n%s", joinedCX)
	}
	for _, want := range []string{
		"hostname SW-FOH-NEW",
		"vlan 20",
		"name Audio_Dante",
		"ip igmp snooping enable",
		"qos dscp-map 56 local-priority 7 name Dante_PTP",
		"interface 1/1/1",
		"description StageBox-1",
		"vlan access 20",
		"mtu 9198",
		"write memory",
	} {
		if !strings.Contains(joinedCX, want) {
			t.Errorf("missing %q in CX template commands:\n%s", want, joinedCX)
		}
	}
	if strings.Contains(joinedCX, "interface 1/1/24") {
		t.Errorf("expected skipped port 1/1/24 to be omitted:\n%s", joinedCX)
	}

	// 2. Cross-compile the same CX template onto an AOS-S (ProCurve) target switch (ports "1".."24")
	targetAOSS := &Snapshot{
		Family: FamilyAOSS,
		Ports:  []Port{{Name: "1"}, {Name: "24"}},
	}
	cmdsAOSS, err := BuildTemplateCommands(FamilyAOSS, targetAOSS, tpl, TemplateApplyRequest{
		NewHostname:      "SW-STAGE-AOSS",
		IncludeVLANs:     true,
		IncludeIGMP:      true,
		IncludeDanteQoS:  true,
		JumboMTU:         true,
		IncludePortVLANs: true,
		IncludePortNames: true,
		Save:             true,
	})
	if err != nil {
		t.Fatalf("BuildTemplateCommands AOS-S failed: %v", err)
	}
	joinedAOSS := strings.Join(cmdsAOSS, "\n")
	for _, want := range []string{
		`hostname "SW-STAGE-AOSS"`,
		"vlan 20",
		`name "Audio_Dante"`,
		"ip igmp",
		"jumbo",
		"qos type-of-service diff-services",
		"interface 1",
		`name "StageBox-1"`,
		"untagged vlan 20",
		"interface 24",
		"tagged vlan 20,30",
	} {
		if !strings.Contains(joinedAOSS, want) {
			t.Errorf("missing %q in AOS-S template commands:\n%s", want, joinedAOSS)
		}
	}

	// 3. Offline console .cfg export with optional new management IP and target port count
	cmdsConsole, err := BuildTemplateCommands(FamilyAOSCX, nil, tpl, TemplateApplyRequest{
		TargetFamily:     FamilyAOSCX,
		TargetPortCount:  8,
		NewHostname:      "SW-CONSOLE-8P",
		MgmtVLAN:         1,
		NewIP:            "10.0.0.55/24",
		NewGateway:       "10.0.0.1",
		IncludeVLANs:     true,
		IncludeDanteQoS:  true,
		IncludePortVLANs: true,
		Save:             true,
	})
	if err != nil {
		t.Fatalf("offline console .cfg build failed: %v", err)
	}
	joinedConsole := strings.Join(cmdsConsole, "\n")
	if !strings.Contains(joinedConsole, "ip address 10.0.0.55/24") || !strings.Contains(joinedConsole, "ip route 0.0.0.0/0 10.0.0.1") {
		t.Fatalf("expected new management IP & gateway in console export, got:\n%s", joinedConsole)
	}
	if strings.Contains(joinedConsole, "interface 1/1/24") {
		t.Fatalf("expected 8-port target not to include port 1/1/24:\n%s", joinedConsole)
	}
}
