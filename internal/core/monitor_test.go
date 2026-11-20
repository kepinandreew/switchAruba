package core

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"myswitch/internal/aruba"
	"myswitch/internal/store"
)

func TestMonitorLiveBounds(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := New(st, nil)

	m.MonitorLive(2)
	if got := m.mon.interval; got != 5*time.Second {
		t.Fatalf("interval 2 clamped = %v, want 5s", got)
	}
	if m.mon.until.IsZero() {
		t.Fatal("expected non-zero until after MonitorLive(2)")
	}

	m.MonitorLive(999)
	if got := m.mon.interval; got != 300*time.Second {
		t.Fatalf("interval 999 clamped = %v, want 300s", got)
	}

	m.MonitorLive(0)
	if !m.mon.until.IsZero() {
		t.Fatalf("until after MonitorLive(0) = %v, want zero", m.mon.until)
	}
}

func TestMonitorRows(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sw1, _ := st.SaveSwitch(store.Switch{Name: "Core-1", Host: "10.0.0.2", Port: 22})
	sw2, _ := st.SaveSwitch(store.Switch{Name: "Web-2", Host: "10.0.0.3", Port: 443, Web: true})
	sw3, _ := st.SaveSwitch(store.Switch{Name: "Edge-3", Host: "10.0.0.4", Port: 22})

	m := New(st, nil)
	m.status[sw1.ID] = &Status{Online: true, Checked: true}
	m.snaps[sw1.ID] = &aruba.Snapshot{
		Family: aruba.FamilyAOSCX,
		Facts:  aruba.Facts{Model: "JL677A", CPU: 14, Memory: 41},
		Ports:  []aruba.Port{{Name: "1/1/1", Up: true}, {Name: "1/1/2", Up: false}},
	}
	m.status[sw2.ID] = &Status{Online: true, Checked: true}
	m.snaps[sw2.ID] = &aruba.Snapshot{Family: aruba.FamilyInstantOn, Web: true}
	m.status[sw3.ID] = &Status{Online: true, Checked: true}
	m.snaps[sw3.ID] = &aruba.Snapshot{
		Family: aruba.FamilyAOSS,
		Facts:  aruba.Facts{Model: "J9775A", CPU: 19, Memory: 52},
	}

	// Inject live reading with >5 ports to verify top-5 sorting and non-null JSON arrays.
	now := time.Now()
	ms := m.monGet(sw1.ID)
	ms.at = now
	ms.cpu = 27
	ms.mem = 45
	ms.rx = 1_500_000
	ms.tx = 500_000
	ms.hasTraffic = true
	ms.hist = []MonSample{{T: now.UnixMilli(), CPU: 27, Mem: 45, Rx: 1_500_000, Tx: 500_000}}
	ms.ports = []MonPort{
		{Port: "1/1/1", Rx: 100, Tx: 50, Up: true},
		{Port: "1/1/2", Rx: 900, Tx: 100, Up: true},
		{Port: "1/1/3", Rx: 0, Tx: 0, Up: false},
		{Port: "1/1/4", Rx: 400, Tx: 200, Up: true},
		{Port: "1/1/5", Rx: 300, Tx: 100, Up: true},
		{Port: "1/1/6", Rx: 200, Tx: 100, Up: true},
		{Port: "1/1/7", Rx: 150, Tx: 100, Up: true},
	}

	rows := m.MonitorRows()
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}
	var r1, r2, r3 MonitorRow
	for _, r := range rows {
		switch r.ID {
		case sw1.ID:
			r1 = r
		case sw2.ID:
			r2 = r
		case sw3.ID:
			r3 = r
		}
	}
	if !r1.Supported || !r1.Live || r1.CPU != 27 || r1.Memory != 45 || r1.PortsUp != 1 || r1.PortsTotal != 2 {
		t.Fatalf("unexpected row 1: %+v", r1)
	}
	if len(r1.Top) != 5 || r1.Top[0].Port != "1/1/2" || r1.Top[4].Port != "1/1/7" {
		t.Fatalf("unexpected top ports: %+v", r1.Top)
	}
	if r2.Supported {
		t.Fatalf("expected web/instant-on switch Supported=false, got true")
	}
	if r3.CPU != 19 || r3.Memory != 52 || r3.Live {
		t.Fatalf("expected snapshot fallback for row 3, got %+v", r3)
	}

	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{`"top":null`, `"history":null`} {
		if strings.Contains(string(raw), bad) {
			t.Errorf("json output contains %s", bad)
		}
	}
}

func TestMapTelemetry(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	swA, _ := st.SaveSwitch(store.Switch{Name: "Core", Host: "10.0.0.1", Port: 22})
	swB, _ := st.SaveSwitch(store.Switch{Name: "Edge", Host: "10.0.0.2", Port: 22})
	m := New(st, nil)
	<-m.loaded

	m.status[swA.ID] = &Status{Online: true, Checked: true}
	m.snaps[swA.ID] = &aruba.Snapshot{
		Family:    aruba.FamilyAOSCX,
		Facts:     aruba.Facts{Hostname: "Core", BaseMAC: "00:00:00:00:00:aa"},
		Ports:     []aruba.Port{{Name: "1/1/24", Up: true, Speed: "10G"}},
		Neighbors: []aruba.Neighbor{{Protocol: "lldp", LocalPort: "1/1/24", SysName: "Edge", MgmtIP: "10.0.0.2", PortID: "24", ChassisID: "00:00:00:00:00:bb"}},
	}
	m.status[swB.ID] = &Status{Online: true, Checked: true}
	m.snaps[swB.ID] = &aruba.Snapshot{
		Family: aruba.FamilyAOSS,
		Facts:  aruba.Facts{Hostname: "Edge", BaseMAC: "00:00:00:00:00:bb"},
		Ports:  []aruba.Port{{Name: "24", Up: true, Speed: "1000FDx"}},
	}

	topo := m.Topology(false)
	if len(topo.Edges) != 1 {
		t.Fatalf("expected 1 topology edge, got %d", len(topo.Edges))
	}
	edge := topo.Edges[0]

	// Put live rates only on Edge (swB) to verify reverse-direction mapping if swB is Edge.To.
	m.mon.mu.Lock()
	msB := m.monGet(swB.ID)
	msB.at = time.Now()
	c1, c5, ci := 18, 15, 92
	msB.cpu, msB.cpu1m, msB.cpu5m, msB.cpuInst, msB.mem, msB.hasTraffic = 18, &c1, &c5, &ci, 42, true
	msB.rx, msB.tx = 2_000_000, 8_000_000
	msB.ports = []MonPort{{Port: "24", Up: true, Rx: 2_000_000, Tx: 8_000_000, Util: 0.8, Speed: 1000}}
	m.mon.mu.Unlock()

	tel := m.MapTelemetry(false)
	lt, ok := tel.Edges[edge.ID]
	if !ok || !lt.Live {
		t.Fatalf("expected live link traffic on %s, got %+v", edge.ID, tel.Edges)
	}
	if edge.From == swB.ID {
		if lt.Rx != 2_000_000 || lt.Tx != 8_000_000 {
			t.Fatalf("expected same direction on From=swB, got %+v", lt)
		}
	} else {
		if lt.Rx != 8_000_000 || lt.Tx != 2_000_000 {
			t.Fatalf("expected reversed direction on To=swB, got %+v", lt)
		}
	}
	nt := tel.Nodes[swB.ID]
	if !nt.Live || nt.CPU != 18 || nt.CPU1m == nil || *nt.CPU1m != 18 || nt.CPU5m == nil || *nt.CPU5m != 15 || nt.CPUInstant == nil || *nt.CPUInstant != 92 {
		t.Fatalf("unexpected node telemetry: %+v", nt)
	}
	for _, r := range m.MonitorRows() {
		if r.ID == swB.ID && (r.CPU != 18 || r.CPU1m == nil || *r.CPU1m != 18 || r.CPUInstant == nil || *r.CPUInstant != 92) {
			t.Fatalf("unexpected monitor row CPU breakdown: %+v", r)
		}
	}
}
