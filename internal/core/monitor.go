package core

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"myswitch/internal/aruba"
	"myswitch/internal/sshx"
	"myswitch/internal/store"
)

// Live monitoring: CPU, memory and traffic of every switch, refreshed every
// few seconds while the Monitor page is open. Each switch gets one SSH
// session that stays open between readings, so the switch doesn't log a new
// login every few seconds; the sessions close when the page is left.

const (
	monMaxHistory = 120 // samples kept per switch
	monLiveFor    = 30 * time.Second
	monParallel   = 8
)

// MonSample is one reading, kept for the sparklines.
type MonSample struct {
	T   int64   `json:"t"` // unix milliseconds
	CPU int     `json:"cpu"`
	Mem int     `json:"mem"`
	Rx  float64 `json:"rx"` // bits per second, all ports together
	Tx  float64 `json:"tx"`
}

// MonPort is the traffic on one port.
type MonPort struct {
	Port  string  `json:"port"`
	Name  string  `json:"name,omitempty"`
	Up    bool    `json:"up"`
	Rx    float64 `json:"rx"`
	Tx    float64 `json:"tx"`
	Util  float64 `json:"util"` // percent of the link speed, -1 if unknown
	Speed int     `json:"speed,omitempty"`
}

// MonitorRow is what the Monitor page shows for one switch.
type MonitorRow struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Host       string      `json:"host"`
	Model      string      `json:"model"`
	Family     string      `json:"family"`
	Online     bool        `json:"online"`
	Disabled   bool        `json:"disabled"`
	Supported  bool        `json:"supported"` // false for Instant On cloud and web switches
	CPU        int         `json:"cpu"`       // -1 when the switch doesn't report it (prefers 1-min avg)
	CPU1m      *int        `json:"cpu1m,omitempty"`
	CPU5m      *int        `json:"cpu5m,omitempty"`
	CPUInstant *int        `json:"cpuInstant,omitempty"`
	Memory     int         `json:"memory"`
	Rx         float64     `json:"rx"`
	Tx         float64     `json:"tx"`
	HasTraffic bool        `json:"hasTraffic"`
	PortsUp    int         `json:"portsUp"`
	PortsTotal int         `json:"portsTotal"`
	Top        []MonPort   `json:"top"`
	History    []MonSample `json:"history"`
	At         time.Time   `json:"at"`   // when the numbers were read
	Live       bool        `json:"live"` // read within the last few intervals
	Error      string      `json:"error,omitempty"`
}

type monSwitch struct {
	sess       *sshx.Session
	busy       bool
	lastPoll   time.Time
	retryAt    time.Time
	prev       []aruba.Counter
	prevAt     time.Time
	at         time.Time
	sysAt      time.Time
	cpu, mem   int
	cpu1m      *int
	cpu5m      *int
	cpuInst    *int
	rx, tx     float64
	hasTraffic bool
	ports      []MonPort
	hist       []MonSample
	err        string
}

type monitorState struct {
	mu       sync.Mutex
	sw       map[string]*monSwitch
	until    time.Time
	interval time.Duration
}

func (m *Manager) monGet(id string) *monSwitch {
	// caller holds m.mon.mu
	if m.mon.sw == nil {
		m.mon.sw = map[string]*monSwitch{}
	}
	ms, ok := m.mon.sw[id]
	if !ok {
		ms = &monSwitch{cpu: -1, mem: -1}
		m.mon.sw[id] = ms
	}
	return ms
}

// MonitorLive keeps live monitoring going for another half minute, reading
// every intervalSec seconds (5 to 300). Zero stops it and closes the sessions.
// The Monitor page calls it every few seconds while it is open.
func (m *Manager) MonitorLive(intervalSec int) {
	m.mon.mu.Lock()
	defer m.mon.mu.Unlock()
	if intervalSec <= 0 {
		m.mon.until = time.Time{}
		return
	}
	if intervalSec < 5 {
		intervalSec = 5
	}
	if intervalSec > 300 {
		intervalSec = 300
	}
	m.mon.interval = time.Duration(intervalSec) * time.Second
	m.mon.until = time.Now().Add(monLiveFor)
}

func (m *Manager) monitorLoop() {
	sem := make(chan struct{}, monParallel)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-m.ctx.Done():
			m.monCloseSessions(true)
			return
		case <-tick.C:
		}
		m.mon.mu.Lock()
		live, iv := time.Now().Before(m.mon.until), m.mon.interval
		m.mon.mu.Unlock()
		if !live {
			m.monCloseSessions(false)
			continue
		}
		for _, sw := range m.st.Switches() {
			if sw.Disabled || sw.Cloud != nil || sw.Web {
				continue
			}
			m.mu.Lock()
			st := m.status[sw.ID]
			skip := st == nil || st.AuthFailed || st.HostKeyChanged || (st.Checked && !st.Online)
			m.mu.Unlock()
			m.mon.mu.Lock()
			ms := m.monGet(sw.ID)
			now := time.Now()
			if skip || ms.busy || now.Before(ms.retryAt) || now.Sub(ms.lastPoll) < iv {
				m.mon.mu.Unlock()
				continue
			}
			select {
			case sem <- struct{}{}:
			default:
				m.mon.mu.Unlock()
				continue
			}
			ms.busy, ms.lastPoll = true, now
			m.mon.mu.Unlock()
			go func(sw store.Switch) {
				defer func() { <-sem }()
				m.monPoll(sw)
			}(sw)
		}
	}
}

// monCloseSessions closes the sessions that aren't in use (all of them when force).
func (m *Manager) monCloseSessions(force bool) {
	m.mon.mu.Lock()
	defer m.mon.mu.Unlock()
	for _, ms := range m.mon.sw {
		if ms.sess != nil && (force || !ms.busy) {
			go ms.sess.Close()
			ms.sess = nil
			ms.prev = nil // the next reading starts a fresh rate
		}
	}
}

func (m *Manager) monFail(id string, err error, dropSession bool, backoff time.Duration) {
	m.mon.mu.Lock()
	defer m.mon.mu.Unlock()
	ms := m.monGet(id)
	ms.err = err.Error()
	ms.busy = false
	ms.retryAt = time.Now().Add(backoff)
	if dropSession && ms.sess != nil {
		go ms.sess.Close()
		ms.sess, ms.prev = nil, nil
	}
}

func (m *Manager) monPoll(sw store.Switch) {
	ctx, cancel := context.WithTimeout(m.ctx, 45*time.Second)
	defer cancel()
	fam := m.family(sw.ID, sw)
	if fam == "" {
		m.monFail(sw.ID, errors.New("not read yet; collect this switch first"), false, 20*time.Second)
		return
	}
	if fam == aruba.FamilyInstantOn {
		m.monFail(sw.ID, errors.New("live traffic isn't available for Instant On"), false, 10*time.Minute)
		return
	}
	m.mon.mu.Lock()
	sess := m.monGet(sw.ID).sess
	m.mon.mu.Unlock()
	if sess == nil {
		s, err := m.connect(ctx, sw)
		if err != nil {
			// a failed login is retried slowly: switches can lock an account after a few of them
			wait := 30 * time.Second
			if strings.Contains(err.Error(), "unable to authenticate") {
				wait = 10 * time.Minute
			}
			m.monFail(sw.ID, err, false, wait)
			return
		}
		aruba.Prepare(ctx, s, fam)
		m.mon.mu.Lock()
		m.monGet(sw.ID).sess = s
		m.mon.mu.Unlock()
		sess = s
	}

	now := time.Now()
	m.mon.mu.Lock()
	msCur := m.monGet(sw.ID)
	prevMem, sysAt := msCur.mem, msCur.sysAt
	m.mon.mu.Unlock()

	var facts aruba.Facts
	var updatedSysAt time.Time
	if fam == aruba.FamilyAOSCX {
		// One command returns 1-min/5-min/instant CPU and memory % together (zero extra overhead).
		resOut, err := sess.Run(ctx, "show system resource-utilization", 15*time.Second)
		if err != nil {
			m.monFail(sw.ID, err, true, 10*time.Second)
			return
		}
		if aruba.IsCLIError(resOut) {
			sysOut, serr := sess.Run(ctx, "show system", 15*time.Second)
			if serr != nil {
				m.monFail(sw.ID, serr, true, 10*time.Second)
				return
			}
			facts = aruba.ParseSystem(sysOut)
			aruba.MergeCPUStats(&facts, aruba.ParseCPUStats(sysOut))
		} else {
			facts = aruba.ParseSystem(resOut)
			aruba.MergeCPUStats(&facts, aruba.ParseCPUStats(resOut))
		}
	} else {
		// AOS-S: keep live polling super light by using tiny "show cpu" (4 lines) on most
		// ticks and refreshing "show system" for memory only ~once per minute.
		facts = aruba.Facts{CPU: -1, Memory: prevMem}
		if prevMem < 0 || sysAt.IsZero() || now.Sub(sysAt) >= 60*time.Second {
			sysOut, err := sess.Run(ctx, "show system", 15*time.Second)
			if err != nil {
				m.monFail(sw.ID, err, true, 10*time.Second)
				return
			}
			facts = aruba.ParseSystem(sysOut)
			aruba.MergeCPUStats(&facts, aruba.ParseCPUStats(sysOut))
			updatedSysAt = now
		}
		cpuOut, cerr := sess.Run(ctx, "show cpu", 10*time.Second)
		if cerr != nil {
			m.monFail(sw.ID, cerr, true, 10*time.Second)
			return
		}
		if !aruba.IsCLIError(cpuOut) {
			aruba.MergeCPUStats(&facts, aruba.ParseCPUStats(cpuOut))
		} else if updatedSysAt.IsZero() {
			// Fallback for legacy switches that don't support "show cpu"
			sysOut, serr := sess.Run(ctx, "show system", 15*time.Second)
			if serr != nil {
				m.monFail(sw.ID, serr, true, 10*time.Second)
				return
			}
			facts = aruba.ParseSystem(sysOut)
			aruba.MergeCPUStats(&facts, aruba.ParseCPUStats(sysOut))
			updatedSysAt = time.Now()
		}
	}

	cmd, alts := aruba.TrafficCommand(fam)
	tout, terr := sess.Run(ctx, cmd, 20*time.Second)
	for _, a := range alts {
		if terr == nil && !aruba.IsCLIError(tout) {
			break
		}
		tout, terr = sess.Run(ctx, a, 20*time.Second)
	}
	if terr != nil {
		m.monFail(sw.ID, terr, true, 10*time.Second)
		return
	}

	now = time.Now()
	var rates []aruba.Rate
	var counters []aruba.Counter
	m.mon.mu.Lock()
	ms := m.monGet(sw.ID)
	prev, prevAt := ms.prev, ms.prevAt
	m.mon.mu.Unlock()
	if aruba.RateIsDirect(fam) {
		rates = aruba.ParseAOSSUtilization(tout)
	} else {
		counters = aruba.ParseCXStatistics(tout)
		if len(prev) > 0 {
			rates = aruba.RatesFromCounters(prev, counters, now.Sub(prevAt).Seconds())
		}
	}

	// port names, link state and speed come from the last full reading
	m.mu.Lock()
	var ports []aruba.Port
	if s := m.snaps[sw.ID]; s != nil {
		ports = s.Ports
	}
	m.mu.Unlock()
	info := make(map[string]aruba.Port, len(ports))
	for _, p := range ports {
		info[p.Name] = p
	}

	var mp []MonPort
	var rx, tx float64
	for _, r := range rates {
		p := info[r.Port]
		speed := aruba.SpeedMbps(p.Speed)
		util := r.Util
		if util < 0 && speed > 0 {
			util = max(r.RxBps, r.TxBps) / (float64(speed) * 1e6) * 100
		}
		rx += r.RxBps
		tx += r.TxBps
		mp = append(mp, MonPort{Port: r.Port, Name: p.Description, Up: p.Up, Rx: r.RxBps, Tx: r.TxBps, Util: util, Speed: speed})
	}
	hasTraffic := len(rates) > 0

	m.mon.mu.Lock()
	defer m.mon.mu.Unlock()
	ms.busy, ms.err, ms.retryAt = false, "", time.Time{}
	if !aruba.RateIsDirect(fam) {
		if len(counters) > 0 {
			ms.prev, ms.prevAt = counters, now
		} else {
			ms.err = "the switch's traffic counters couldn't be read (see Raw output of this switch)"
		}
	} else if !hasTraffic {
		ms.err = "the switch's traffic table couldn't be read (see Raw output of this switch)"
	}
	ms.at, ms.cpu, ms.mem = now, facts.CPU, facts.Memory
	ms.cpu1m, ms.cpu5m, ms.cpuInst = facts.CPU1m, facts.CPU5m, facts.CPUInstant
	if !updatedSysAt.IsZero() {
		ms.sysAt = updatedSysAt
	}
	if hasTraffic {
		ms.rx, ms.tx, ms.hasTraffic, ms.ports = rx, tx, true, mp
	}
	ms.hist = append(ms.hist, MonSample{T: now.UnixMilli(), CPU: facts.CPU, Mem: facts.Memory, Rx: ms.rx, Tx: ms.tx})
	if n := len(ms.hist); n > monMaxHistory {
		ms.hist = append([]MonSample(nil), ms.hist[n-monMaxHistory:]...)
	}
}

// MonitorRows lists every switch with its latest live numbers; before a live
// reading exists, CPU and memory come from the last full collection.
func (m *Manager) MonitorRows() []MonitorRow {
	sws := m.st.Switches()
	m.mon.mu.Lock()
	iv := m.mon.interval
	m.mon.mu.Unlock()
	if iv == 0 {
		iv = 10 * time.Second
	}
	rows := make([]MonitorRow, 0, len(sws))
	for _, sw := range sws {
		m.mu.Lock()
		st := m.status[sw.ID]
		snap := m.snaps[sw.ID]
		online := st != nil && st.Online
		m.mu.Unlock()
		row := MonitorRow{ID: sw.ID, Name: sw.Name, Host: sw.Host, Online: online, Disabled: sw.Disabled, CPU: -1, Memory: -1,
			Supported: sw.Cloud == nil && !sw.Web, Top: []MonPort{}, History: []MonSample{}}
		if snap != nil {
			row.Model, row.Family = snap.Facts.Model, snap.Family.Label()
			if row.Name == "" {
				row.Name = snap.Facts.Hostname
			}
			row.CPU, row.Memory = snap.Facts.CPU, snap.Facts.Memory
			row.CPU1m, row.CPU5m, row.CPUInstant = snap.Facts.CPU1m, snap.Facts.CPU5m, snap.Facts.CPUInstant
			row.PortsTotal = len(snap.Ports)
			for _, p := range snap.Ports {
				if p.Up {
					row.PortsUp++
				}
			}
			if snap.Family == aruba.FamilyInstantOn {
				row.Supported = false
			}
		}
		if row.Name == "" {
			row.Name = sw.Host
		}
		m.mon.mu.Lock()
		if ms := m.mon.sw[sw.ID]; ms != nil {
			row.Error = ms.err
			if !ms.at.IsZero() {
				row.At = ms.at
				row.Live = time.Since(ms.at) < 3*iv+10*time.Second
				if ms.cpu >= 0 {
					row.CPU = ms.cpu
					row.CPU1m, row.CPU5m, row.CPUInstant = ms.cpu1m, ms.cpu5m, ms.cpuInst
				}
				if ms.mem >= 0 {
					row.Memory = ms.mem
				}
				row.HasTraffic, row.Rx, row.Tx = ms.hasTraffic, ms.rx, ms.tx
				row.History = append(row.History, ms.hist...)
				top := append([]MonPort(nil), ms.ports...)
				sort.Slice(top, func(i, j int) bool { return top[i].Rx+top[i].Tx > top[j].Rx+top[j].Tx })
				for _, p := range top {
					if len(row.Top) == 5 || p.Rx+p.Tx <= 0 {
						break
					}
					row.Top = append(row.Top, p)
				}
			}
		}
		m.mon.mu.Unlock()
		rows = append(rows, row)
	}
	return rows
}

// LinkTraffic is live throughput along one topology edge (Tx: From -> To, Rx: To -> From).
type LinkTraffic struct {
	Rx    float64 `json:"rx"`
	Tx    float64 `json:"tx"`
	Util  float64 `json:"util"`
	Speed int     `json:"speed"`
	Live  bool    `json:"live"`
}

// NodeTraffic summarizes live CPU, memory, and total switch traffic for a map node.
type NodeTraffic struct {
	CPU        int     `json:"cpu"`
	CPU1m      *int    `json:"cpu1m,omitempty"`
	CPU5m      *int    `json:"cpu5m,omitempty"`
	CPUInstant *int    `json:"cpuInstant,omitempty"`
	Memory     int     `json:"memory"`
	Rx         float64 `json:"rx"`
	Tx         float64 `json:"tx"`
	HasTraffic bool    `json:"hasTraffic"`
	Live       bool    `json:"live"`
}

// MapTelemetry pairs live per-link and per-switch metrics for the Network map.
type MapTelemetry struct {
	Edges map[string]LinkTraffic `json:"edges"`
	Nodes map[string]NodeTraffic `json:"nodes"`
}

func findMonPort(ports []MonPort, name string) (MonPort, bool) {
	if name == "" {
		return MonPort{}, false
	}
	for _, p := range ports {
		if p.Port == name {
			return p, true
		}
	}
	low := strings.ToLower(strings.TrimSpace(name))
	for _, p := range ports {
		if strings.ToLower(p.Port) == low {
			return p, true
		}
	}
	return MonPort{}, false
}

// MapTelemetry resolves live traffic on each topology link and switch node from
// the current monitor cache without blocking on SSH.
func (m *Manager) MapTelemetry(showEndpoints bool) MapTelemetry {
	topo := m.Topology(showEndpoints)
	out := MapTelemetry{
		Edges: map[string]LinkTraffic{},
		Nodes: map[string]NodeTraffic{},
	}
	m.mon.mu.Lock()
	defer m.mon.mu.Unlock()
	iv := m.mon.interval
	if iv == 0 {
		iv = 10 * time.Second
	}
	fresh := func(ms *monSwitch) bool {
		return ms != nil && !ms.at.IsZero() && time.Since(ms.at) < 3*iv+10*time.Second
	}
	for id, ms := range m.mon.sw {
		if ms == nil || ms.at.IsZero() {
			continue
		}
		out.Nodes[id] = NodeTraffic{
			CPU:        ms.cpu,
			CPU1m:      ms.cpu1m,
			CPU5m:      ms.cpu5m,
			CPUInstant: ms.cpuInst,
			Memory:     ms.mem,
			Rx:         ms.rx,
			Tx:         ms.tx,
			HasTraffic: ms.hasTraffic,
			Live:       fresh(ms),
		}
	}
	for _, e := range topo.Edges {
		if ms := m.mon.sw[e.From]; fresh(ms) && ms.hasTraffic {
			if p, ok := findMonPort(ms.ports, e.FromPort); ok {
				out.Edges[e.ID] = LinkTraffic{Rx: p.Rx, Tx: p.Tx, Util: p.Util, Speed: p.Speed, Live: true}
				continue
			}
		}
		if ms := m.mon.sw[e.To]; fresh(ms) && ms.hasTraffic {
			if p, ok := findMonPort(ms.ports, e.ToPort); ok {
				// Reversed perspective: Rx on To's port arrives from From (Tx on edge).
				out.Edges[e.ID] = LinkTraffic{Rx: p.Tx, Tx: p.Rx, Util: p.Util, Speed: p.Speed, Live: true}
			}
		}
	}
	return out
}
