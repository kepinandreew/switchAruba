package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"myswitch/internal/aruba"
	"myswitch/internal/core"
	"myswitch/internal/netbind"
	"myswitch/internal/store"
)

// App is the object bound to the frontend; every exported method is callable
// from JavaScript as window.go.main.App.<Method>.
type App struct {
	ctx context.Context
	st  *store.Store
	mgr *core.Manager

	emitMu   sync.Mutex
	emitWait bool

	swapMu sync.Mutex // held while the data folder is being swapped
	notice string     // shown once in the UI after startup

	lockMu       sync.Mutex
	screenLocked bool // locked while running; the data key stays in memory
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	dir, _, notice, err := store.ResolveDir()
	a.notice = notice
	if err == nil {
		a.st, err = store.Open(dir)
	}
	if err != nil {
		runtime.MessageDialog(ctx, runtime.MessageDialogOptions{Type: runtime.ErrorDialog, Title: "MySwitch", Message: "Can't open data folder: " + err.Error()})
		os.Exit(1)
	}
	if set := a.st.Settings(); set.BindIP != "" {
		netbind.SetBindIP(set.BindIP)
	}
	if !a.st.Locked() {
		a.mgr = core.New(a.st, a.emit)
		a.mgr.Start()
	}
	// with an app password nothing is read or polled until it's entered
	go a.updateLoop()
}

// ---- optional app password (issue #13) ----

type LockInfo struct {
	Enabled   bool `json:"enabled"`   // an app password is set
	Locked    bool `json:"locked"`    // the password has to be entered now
	Starting  bool `json:"starting"`  // locked since start: nothing has been read yet
	LockAfter int  `json:"lockAfter"` // idle minutes before the window locks; 0 never
}

func (a *App) AppLock() LockInfo {
	a.lockMu.Lock()
	defer a.lockMu.Unlock()
	starting := a.st.Locked()
	return LockInfo{Enabled: a.st.HasPassword(), Locked: starting || a.screenLocked, Starting: starting, LockAfter: a.st.Settings().LockAfter}
}

// Unlock checks the app password. At startup it opens the data and starts
// polling; later it only unlocks the window.
func (a *App) Unlock(pass string) error {
	a.lockMu.Lock()
	defer a.lockMu.Unlock()
	if a.st.Locked() {
		if err := a.st.Unlock(pass); err != nil {
			time.Sleep(time.Second) // slow down guessing
			return err
		}
		a.mgr = core.New(a.st, a.emit)
		a.mgr.Start()
		a.screenLocked = false
		return nil
	}
	if a.screenLocked && !a.st.CheckPassword(pass) {
		time.Sleep(time.Second)
		return store.ErrWrongPassword
	}
	a.screenLocked = false
	return nil
}

// LockNow locks the window. Polling goes on in the background.
func (a *App) LockNow() {
	a.lockMu.Lock()
	defer a.lockMu.Unlock()
	if a.st.HasPassword() {
		a.screenLocked = true
	}
}

// SetAppPassword sets, changes or (with next empty) removes the app password.
func (a *App) SetAppPassword(current, next string) error {
	a.lockMu.Lock()
	defer a.lockMu.Unlock()
	return a.st.SetPassword(current, next)
}

// ForgetAppPassword starts over without the password, keeping switches and
// history but clearing every saved switch password. Only offered while the
// app is locked at startup.
func (a *App) ForgetAppPassword() error {
	a.lockMu.Lock()
	defer a.lockMu.Unlock()
	if !a.st.Locked() {
		return errors.New("MySwitch isn't locked")
	}
	if err := a.st.ForgetPassword(); err != nil {
		return err
	}
	a.mgr = core.New(a.st, a.emit)
	a.mgr.Start()
	return nil
}

func (a *App) shutdown(ctx context.Context) {
	if a.mgr != nil {
		a.mgr.Stop()
	}
}

// emit coalesces bursts of change notifications into one UI refresh.
func (a *App) emit(string) {
	a.emitMu.Lock()
	if a.emitWait {
		a.emitMu.Unlock()
		return
	}
	a.emitWait = true
	a.emitMu.Unlock()
	time.AfterFunc(400*time.Millisecond, func() {
		a.emitMu.Lock()
		a.emitWait = false
		a.emitMu.Unlock()
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "changed")
		}
	})
}

func (a *App) Version() string  { return Version }
func (a *App) Platform() string { return goruntime.GOOS }

// ---- views ----

func (a *App) GetDashboard() core.Dashboard { return a.mgr.Dashboard() }

func (a *App) GetSwitch(id string) (core.Detail, error) { return a.mgr.Detail(id) }

func (a *App) GetTopology(showEndpoints bool) core.Topology { return a.mgr.Topology(showEndpoints) }

// FindMAC finds the switch and port a client is plugged into, by MAC address or part of one.
func (a *App) FindMAC(query string) []core.MACHit { return a.mgr.FindMAC(query) }

// Clients lists every learned MAC address once, at the port the client is plugged into.
func (a *App) Clients() []core.ClientEntry { return a.mgr.Clients() }

func (a *App) SaveLayout(l map[string]store.Point) error { return a.st.SaveLayout(l) }

// ---- inventory ----

func (a *App) SaveSwitch(sw store.Switch) (store.Switch, error) {
	isNew := sw.ID == ""
	saved, err := a.st.SaveSwitch(sw)
	if err != nil {
		return saved, err
	}
	a.mgr.ClearAuthFailure(saved.ID)
	if isNew {
		go func() {
			a.mgr.Enqueue(saved.ID)
		}()
	}
	a.emit("")
	return saved, nil
}

type BulkResult struct {
	Added  int      `json:"added"`
	Errors []string `json:"errors"`
}

// BulkAdd takes one address per line (optionally "name,address" or "address name"),
// IP ranges (e.g. 10.0.0.2 - 10.0.0.150, 10.0.0.2-150, 10.0.0.2 to 10.0.0.10),
// or CIDR subnets (e.g. 10.0.0.0/24).
func (a *App) BulkAdd(text, credentialID, site string, family string) BulkResult {
	res := BulkResult{}
	for _, raw := range strings.Split(text, "\n") {
		name, hosts := parseBulkLine(raw)
		for _, h := range hosts {
			sw := store.Switch{Name: name, Host: h, Port: 22, CredentialID: credentialID, Site: site, Family: aruba.Family(family)}
			if sw.Family == aruba.FamilyInstantOn {
				// Instant On switches have no SSH CLI: read them through their web interface
				sw.Web, sw.Port = true, 443
			}
			_, err := a.st.SaveSwitch(sw)
			if err != nil {
				res.Errors = append(res.Errors, h+": "+err.Error())
				continue
			}
			res.Added++
		}
	}
	if res.Added > 0 {
		go a.mgr.RefreshAll()
		a.emit("")
	}
	return res
}

func parseBulkLine(raw string) (name string, hosts []string) {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", nil
	}

	// 1. "name, host" or "host, name"
	if strings.Contains(line, ",") {
		parts := strings.SplitN(line, ",", 2)
		p1, p2 := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if net.ParseIP(p1) != nil && net.ParseIP(p2) == nil {
			return p2, expandHostOrRange(p1)
		}
		return p1, expandHostOrRange(p2)
	}

	// 2. Check CIDR notation, e.g. 10.0.0.0/24
	if strings.Contains(line, "/") {
		if cidrHosts := expandCIDR(line); len(cidrHosts) > 0 {
			return "", cidrHosts
		}
	}

	// 3. Check Range: hyphen, "to", or ".."
	// E.g. "10.0.0.2 - 10.0.0.150", "10.0.0.2 - 150", "10.0.0.2-150"
	if rngHosts := expandHostOrRange(line); len(rngHosts) > 1 {
		return "", rngHosts
	}

	// 4. "host name" or "name host"
	if f := strings.Fields(line); len(f) == 2 {
		if net.ParseIP(f[0]) != nil && net.ParseIP(f[1]) == nil {
			return f[1], []string{f[0]}
		} else if net.ParseIP(f[1]) != nil && net.ParseIP(f[0]) == nil {
			return f[0], []string{f[1]}
		}
	}

	return "", []string{line}
}

func expandHostOrRange(str string) []string {
	str = strings.TrimSpace(str)

	var from, to string
	var found bool

	if strings.Contains(str, " to ") {
		parts := strings.SplitN(str, " to ", 2)
		from, to, found = parts[0], parts[1], true
	} else if strings.Contains(str, "..") {
		parts := strings.SplitN(str, "..", 2)
		from, to, found = parts[0], parts[1], true
	} else if strings.Contains(str, "-") {
		parts := strings.SplitN(str, "-", 2)
		from, to, found = parts[0], parts[1], true
	}

	if found {
		from = strings.TrimSpace(from)
		to = strings.TrimSpace(to)
		ipFrom := net.ParseIP(from).To4()
		if ipFrom != nil {
			return expandIPv4Range(ipFrom, to)
		}
	}

	return []string{str}
}

func expandIPv4Range(ipFrom net.IP, toStr string) []string {
	fromInt := binary.BigEndian.Uint32(ipFrom)
	var toInt uint32

	toStr = strings.TrimSpace(toStr)
	// Case 1: toStr is a single number, e.g. "150"
	if n, err := strconv.Atoi(toStr); err == nil && n >= 0 && n <= 255 {
		endIP := make(net.IP, 4)
		copy(endIP, ipFrom)
		endIP[3] = byte(n)
		toInt = binary.BigEndian.Uint32(endIP)
	} else if ipTo := net.ParseIP(toStr).To4(); ipTo != nil {
		// Case 2: toStr is a full IP, e.g. "10.0.0.150"
		toInt = binary.BigEndian.Uint32(ipTo)
	} else {
		return []string{ipFrom.String()}
	}

	if toInt < fromInt {
		fromInt, toInt = toInt, fromInt
	}

	count := toInt - fromInt + 1
	if count > 1024 {
		count = 1024
	}

	out := make([]string, 0, count)
	for cur := fromInt; cur < fromInt+count; cur++ {
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, cur)
		out = append(out, fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3]))
	}
	return out
}

func expandCIDR(cidrStr string) []string {
	_, ipNet, err := net.ParseCIDR(strings.TrimSpace(cidrStr))
	if err != nil || ipNet.IP.To4() == nil {
		return nil
	}
	mask := binary.BigEndian.Uint32(ipNet.Mask)
	start := binary.BigEndian.Uint32(ipNet.IP.To4())
	total := ^mask + 1

	var first, last uint32
	if total <= 2 {
		first = start
		last = start + total - 1
	} else {
		first = start + 1
		last = start + total - 2
	}

	if last < first {
		return nil
	}

	if last-first+1 > 1024 {
		last = first + 1023
	}

	var out []string
	for cur := first; cur <= last; cur++ {
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, cur)
		out = append(out, fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3]))
	}
	return out
}

// DeleteOffline removes all switches that are currently unreachable / offline.
// If site is specified, only offline switches on that site are deleted.
func (a *App) DeleteOffline(site string) (int, error) {
	if a.st == nil {
		return 0, errors.New("storage not initialized")
	}
	dash := a.mgr.Dashboard()
	var toDelete []string
	for _, sw := range dash.Switches {
		if sw.Disabled {
			continue
		}
		if site != "" && sw.Site != site {
			continue
		}
		// Consider offline if checked and not online, or never reached online and not currently collecting
		isOffline := (sw.Status.Checked && !sw.Status.Online) || (!sw.Status.Online && sw.Summary == nil)
		if isOffline && !sw.Status.Collecting {
			toDelete = append(toDelete, sw.ID)
		}
	}
	for _, id := range toDelete {
		_ = a.st.DeleteSwitch(id)
		a.mgr.Forget(id)
	}
	if len(toDelete) > 0 {
		a.emit("")
	}
	return len(toDelete), nil
}

func (a *App) DeleteSwitch(id string) error {
	if err := a.st.DeleteSwitch(id); err != nil {
		return err
	}
	a.mgr.Forget(id)
	a.emit("")
	return nil
}

// DeleteSwitches removes several switches and their saved data.
func (a *App) DeleteSwitches(ids []string) error {
	for _, id := range ids {
		if err := a.st.DeleteSwitch(id); err != nil {
			return err
		}
		a.mgr.Forget(id)
	}
	a.emit("")
	return nil
}

// SetDisabled pauses (no polling, no SSH, no alerts) or resumes switches.
func (a *App) SetDisabled(ids []string, disabled bool) error {
	if err := a.st.SetDisabled(ids, disabled); err != nil {
		return err
	}
	if !disabled {
		go func() {
			for _, id := range ids {
				a.mgr.Resume(id)
			}
		}()
	}
	a.emit("")
	return nil
}

func (a *App) TestSwitch(sw store.Switch) (string, error) { return a.mgr.TestConnection(sw) }

// UseWebInterface moves Instant On switches added for SSH over to their web
// interface, and reads them again. ids empty means every such switch.
func (a *App) UseWebInterface(ids []string) (int, error) {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	n := 0
	for _, sw := range a.st.Switches() {
		if sw.Web || sw.Cloud != nil || sw.Family != aruba.FamilyInstantOn || (len(ids) > 0 && !want[sw.ID]) {
			continue
		}
		sw.Web, sw.Port = true, 443
		if _, err := a.st.SaveSwitch(sw); err != nil {
			return n, err
		}
		a.mgr.ClearAuthFailure(sw.ID)
		go a.mgr.Enqueue(sw.ID)
		n++
	}
	a.emit("")
	return n, nil
}

// DiscoverCloud lists the switches on an Instant On account's sites.
func (a *App) DiscoverCloud(credentialID string) ([]core.CloudSite, error) {
	return a.mgr.DiscoverCloud(credentialID)
}

// AddCloud adds Instant On switches picked from DiscoverCloud.
func (a *App) AddCloud(credentialID string, picks []core.CloudPick) (int, error) {
	n, err := a.mgr.AddCloud(credentialID, picks)
	a.emit("")
	return n, err
}

func (a *App) Refresh(id string) { a.mgr.Enqueue(id) }

func (a *App) RefreshAll() { a.mgr.RefreshAll() }

func (a *App) TrustHostKey(id string) { a.mgr.TrustNewHostKey(id) }

// ---- credentials & settings ----

func (a *App) Credentials() []store.Credential { return a.st.Credentials() }

func (a *App) SaveCredential(c store.Credential) (store.Credential, error) {
	out, err := a.st.SaveCredential(c)
	if err == nil {
		for _, sw := range a.st.Switches() {
			if sw.CredentialID == out.ID {
				a.mgr.ClearAuthFailure(sw.ID)
			}
		}
	}
	return out, err
}

func (a *App) DeleteCredential(id string) error { return a.st.DeleteCredential(id) }

func (a *App) GetSettings() store.Settings { return a.st.Settings() }

func (a *App) SaveSettings(s store.Settings) error {
	if s.UIScale != 0 && (s.UIScale < 75 || s.UIScale > 150) {
		s.UIScale = 100
	}
	if s.UIFont != "" && s.UIFont != "system" && s.UIFont != "verdana" {
		s.UIFont = ""
	}
	err := a.st.SaveSettings(s)
	a.emit("")
	return err
}

// ---- actions on a switch ----

func (a *App) RunCommand(id, cmd string) (string, error) { return a.mgr.RunCommand(id, cmd) }

func (a *App) PreviewVLAN(id string, ch aruba.VLANChange) ([]string, error) {
	return a.mgr.PreviewVLAN(id, ch)
}

func (a *App) ApplyVLAN(id string, ch aruba.VLANChange) (aruba.ApplyResult, error) {
	return a.mgr.ApplyVLAN(id, ch)
}

func (a *App) PreviewPortVLAN(id string, ch aruba.PortVLANChange) ([]string, error) {
	return a.mgr.PreviewPortVLAN(id, ch)
}

func (a *App) ApplyPortVLAN(id string, ch aruba.PortVLANChange) (aruba.ApplyResult, error) {
	return a.mgr.ApplyPortVLAN(id, ch)
}

// MonitorLive keeps live CPU and traffic readings going; the Monitor page calls it
// every few seconds while it is open, and with 0 when it is left.
func (a *App) MonitorLive(intervalSec int) { a.mgr.MonitorLive(intervalSec) }

// MonitorRows returns the latest CPU, memory and traffic of every switch.
func (a *App) MonitorRows() []core.MonitorRow { return a.mgr.MonitorRows() }

// GetMapTelemetry returns live per-link and per-switch metrics for Network map animation.
func (a *App) GetMapTelemetry(showEndpoints bool) core.MapTelemetry {
	return a.mgr.MapTelemetry(showEndpoints)
}

func (a *App) PreviewPortPoE(id string, ch aruba.PortPoEChange) ([]string, error) {
	return a.mgr.PreviewPortPoE(id, ch)
}

func (a *App) ApplyPortPoE(id string, ch aruba.PortPoEChange) (aruba.ApplyResult, error) {
	return a.mgr.ApplyPortPoE(id, ch)
}

func (a *App) PreviewPortAdmin(id string, ch aruba.PortAdminChange) ([]string, error) {
	return a.mgr.PreviewPortAdmin(id, ch)
}

func (a *App) ApplyPortAdmin(id string, ch aruba.PortAdminChange) (aruba.ApplyResult, error) {
	return a.mgr.ApplyPortAdmin(id, ch)
}

func (a *App) PreviewDanteProfile(id string, ch aruba.DanteProfileChange) ([]string, error) {
	return a.mgr.PreviewDanteProfile(id, ch)
}

func (a *App) ApplyDanteProfile(id string, ch aruba.DanteProfileChange) (aruba.ApplyResult, error) {
	return a.mgr.ApplyDanteProfile(id, ch)
}

func (a *App) PreviewPortAccess(id string, ch aruba.PortAccessChange) ([]string, error) {
	return a.mgr.PreviewPortAccess(id, ch)
}

func (a *App) ApplyPortAccess(id string, ch aruba.PortAccessChange) (aruba.ApplyResult, error) {
	return a.mgr.ApplyPortAccess(id, ch)
}

// PortNameSuggestions lists names for ports from their LLDP neighbors.
func (a *App) PortNameSuggestions(id string) []aruba.PortNameSuggestion {
	return a.mgr.PortNameSuggestions(id)
}

func (a *App) PreviewPortNames(id string, ch aruba.PortNameChange) ([]string, error) {
	return a.mgr.PreviewPortNames(id, ch)
}

func (a *App) ApplyPortNames(id string, ch aruba.PortNameChange) (aruba.ApplyResult, error) {
	return a.mgr.ApplyPortNames(id, ch)
}

// SetFirmwareHold keeps switches on their firmware, or lets them be updated again.
func (a *App) SetFirmwareHold(ids []string, hold bool) error {
	if err := a.st.SetFirmwareHold(ids, hold); err != nil {
		return err
	}
	a.emit("")
	return nil
}

// SaveConfig runs write memory on the switch.
func (a *App) SaveConfig(id string) error { return a.mgr.SaveConfig(id) }

func (a *App) RefreshPortClients(id string) ([]aruba.PortClient, error) {
	return a.mgr.RefreshPortClients(id)
}

func (a *App) GetBackup(id, name string) (string, error) { return a.st.ReadBackup(id, name) }

// ExportText asks where to save and writes the content (configs, raw output).
func (a *App) ExportText(defaultName, content string) (string, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{DefaultFilename: defaultName, Title: "Save"})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", errors.New("cancelled")
	}
	return path, os.WriteFile(path, []byte(content), 0o644)
}

func (a *App) OpenURL(u string) {
	if strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://") {
		runtime.BrowserOpenURL(a.ctx, u)
	}
}

func (a *App) VLANOverview() []core.SwitchVLANs { return a.mgr.VLANOverview() }

// SearchConfigs searches every switch's in-memory running-config (zero SSH calls).
func (a *App) SearchConfigs(query string, useRegex bool, site string) ([]core.ConfigSearchHit, error) {
	return a.mgr.SearchConfigs(query, useRegex, site)
}

// RunCableDiag runs an on-demand copper TDR cable diagnostic test on a single port.
func (a *App) RunCableDiag(id, port string) (aruba.CableDiagResult, error) {
	return a.mgr.RunCableDiag(id, port)
}

// ---- network interface (NIC) selection & fast subnet discovery ----

func (a *App) ListNICs() []netbind.NICInfo {
	return netbind.ListNICs()
}

func (a *App) GetActiveNIC() string {
	return netbind.GetBindIP()
}

func (a *App) SetActiveNIC(ip, name string, skipStartup bool) error {
	netbind.SetBindIP(ip)
	if a.st == nil {
		return nil
	}
	set := a.st.Settings()
	set.BindIP = netbind.GetBindIP()
	set.BindNIC = strings.TrimSpace(name)
	set.SkipStartupNIC = skipStartup
	return a.st.SaveSettings(set)
}

func (a *App) ScanSubnet(target string) ([]netbind.DiscoveredHost, error) {
	existing := map[string][2]string{}
	if a.st != nil {
		for _, sw := range a.st.Switches() {
			lbl := sw.Name
			if lbl == "" {
				lbl = sw.Host
			}
			existing[sw.Host] = [2]string{sw.ID, lbl}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return netbind.ScanSubnet(ctx, target, existing)
}

// ---- switch configuration templates (IP-free provisioning) ----

func (a *App) Templates() []aruba.SwitchTemplate {
	if a.st == nil {
		return nil
	}
	return a.st.Templates()
}

func (a *App) SaveSwitchAsTemplate(switchID, name, notes string) (aruba.SwitchTemplate, error) {
	tpl, err := a.mgr.ExtractSwitchTemplate(switchID, name, notes)
	if err == nil {
		a.emit("")
	}
	return tpl, err
}

func (a *App) SaveConfigTextAsTemplate(name, notes, rawCfg string) (aruba.SwitchTemplate, error) {
	tpl, err := aruba.ExtractTemplateFromConfigText(name, notes, rawCfg)
	if err != nil {
		return tpl, err
	}
	saved, err := a.st.SaveTemplate(tpl)
	if err == nil {
		a.emit("")
	}
	return saved, err
}

func (a *App) SaveTemplate(tpl aruba.SwitchTemplate) (aruba.SwitchTemplate, error) {
	saved, err := a.st.SaveTemplate(tpl)
	if err == nil {
		a.emit("")
	}
	return saved, err
}

func (a *App) DeleteTemplate(id string) error {
	err := a.st.DeleteTemplate(id)
	if err == nil {
		a.emit("")
	}
	return err
}

func (a *App) PreviewTemplate(switchID string, req aruba.TemplateApplyRequest) ([]string, error) {
	return a.mgr.PreviewTemplate(switchID, req)
}

func (a *App) ApplyTemplate(switchID string, req aruba.TemplateApplyRequest) (aruba.ApplyResult, error) {
	return a.mgr.ApplyTemplate(switchID, req)
}

// GenerateTemplateConfig compiles a SwitchTemplate into an offline .cfg / Console CLI script
// (for pasting over a serial Console cable or saving as a .cfg file for a brand-new switch).
func (a *App) GenerateTemplateConfig(req aruba.TemplateApplyRequest) ([]string, error) {
	if a.st == nil {
		return nil, errors.New("storage not initialized")
	}
	tpl, ok := a.st.Template(req.TemplateID)
	if !ok {
		return nil, errors.New("template not found")
	}
	fam := req.TargetFamily
	if fam != aruba.FamilyAOSCX && fam != aruba.FamilyAOSS {
		if tpl.SourceFamily == aruba.FamilyAOSCX || tpl.SourceFamily == aruba.FamilyAOSS {
			fam = tpl.SourceFamily
		} else {
			fam = aruba.FamilyAOSCX
		}
	}
	return aruba.BuildTemplateCommands(fam, nil, tpl, req)
}
