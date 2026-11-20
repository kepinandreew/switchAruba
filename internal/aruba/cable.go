package aruba

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// CablePair represents one twisted wire pair reading from a TDR cable diagnostic test.
type CablePair struct {
	Pair      string `json:"pair"`                // "1-2", "3-6", "4-5", "7-8"
	Status    string `json:"status"`              // "good", "OK", "open", "short", "intra-short", "inter-short", "high-imp", ...
	OK        bool   `json:"ok"`                  // true when pair status is good/OK
	Impedance string `json:"impedance,omitempty"` // e.g. "85-115", ">300" (AOS-CX)
	Distance  string `json:"distance,omitempty"`  // e.g. "10 m", "4 +/- 5 m", "34m"
	MDIMode   string `json:"mdiMode,omitempty"`   // "mdi", "mdix" (AOS-CX)
}

// CableDiagResult is the parsed output of a copper TDR cable diagnostic test.
type CableDiagResult struct {
	Port      string      `json:"port"`
	Supported bool        `json:"supported"`
	Health    string      `json:"health"` // "ok", "open", "fault", "unknown"
	Summary   string      `json:"summary"`
	Pairs     []CablePair `json:"pairs"`
	Raw       string      `json:"raw"`
	TestedAt  time.Time   `json:"testedAt"`
	Commands  []string    `json:"commands"`
	Error     string      `json:"error,omitempty"`
}

// cableWait controls polling delay between test trigger and result read (overridden in tests).
var cableWait = 1500 * time.Millisecond

// RunCableDiag executes an on-demand TDR cable diagnostic test on a single copper port.
// Because TDR interrupts link on the tested port, it is strictly manual / on-demand.
func RunCableDiag(ctx context.Context, r Runner, f Family, port string) (CableDiagResult, error) {
	port = strings.TrimSpace(port)
	if port == "" || !safePort.MatchString(port) {
		return CableDiagResult{}, fmt.Errorf("invalid port %q", port)
	}
	if f == FamilyInstantOn {
		return CableDiagResult{}, fmt.Errorf("cable diagnostics over SSH aren't supported on Instant On switches")
	}

	var cmds []string
	var rawParts []string

	switch f {
	case FamilyAOSCX:
		testCmd := "diag cable-diagnostic test " + port
		cmds = append(cmds, testCmd)
		testOut, err := r.Run(ctx, testCmd, 35*time.Second)
		if testOut != "" {
			rawParts = append(rawParts, "> "+testCmd+"\n"+testOut)
		}
		if err != nil {
			return CableDiagResult{Port: port, Raw: strings.Join(rawParts, "\n\n"), Commands: cmds, TestedAt: time.Now(), Error: err.Error()}, err
		}
		if cableUnsupported(testOut) {
			res := ParseCableDiag(port, strings.Join(rawParts, "\n\n"))
			res.Commands = cmds
			return res, nil
		}

		showCmd := "diag cable-diagnostic show " + port
		cmds = append(cmds, showCmd)
		var showOut string
		for attempt := 0; attempt < 4; attempt++ {
			if attempt > 0 || !strings.Contains(strings.ToLower(testOut), "1-2") {
				select {
				case <-ctx.Done():
					return CableDiagResult{}, ctx.Err()
				case <-time.After(cableWait):
				}
			}
			out, err := r.Run(ctx, showCmd, 20*time.Second)
			if err != nil {
				return CableDiagResult{Port: port, Raw: strings.Join(rawParts, "\n\n"), Commands: cmds, TestedAt: time.Now(), Error: err.Error()}, err
			}
			if IsCLIError(out) && attempt == 0 {
				altCmd := "show diag cable-diagnostic " + port
				out2, err2 := r.Run(ctx, altCmd, 20*time.Second)
				if err2 == nil && !IsCLIError(out2) {
					showCmd = altCmd
					cmds[len(cmds)-1] = altCmd
					out = out2
				}
			}
			showOut = out
			low := strings.ToLower(out)
			if !strings.Contains(low, "in-progress") && !strings.Contains(low, "in progress") && !strings.Contains(low, "testing") {
				break
			}
		}
		if showOut != "" {
			rawParts = append(rawParts, "> "+showCmd+"\n"+showOut)
		}

	default: // AOS-S
		testCmd := "test cable-diagnostics " + port
		cmds = append(cmds, testCmd)
		testOut, err := r.Run(ctx, testCmd, 40*time.Second)
		if testOut != "" {
			rawParts = append(rawParts, "> "+testCmd+"\n"+testOut)
		}
		if err != nil {
			return CableDiagResult{Port: port, Raw: strings.Join(rawParts, "\n\n"), Commands: cmds, TestedAt: time.Now(), Error: err.Error()}, err
		}
		if !cableUnsupported(testOut) && !hasCablePairs(testOut) {
			select {
			case <-ctx.Done():
				return CableDiagResult{}, ctx.Err()
			case <-time.After(cableWait):
			}
			showCmd := "show cable-diagnostics"
			cmds = append(cmds, showCmd)
			showOut, err := r.Run(ctx, showCmd, 20*time.Second)
			if err == nil && showOut != "" {
				rawParts = append(rawParts, "> "+showCmd+"\n"+showOut)
			}
		}
	}

	raw := strings.Join(rawParts, "\n\n")
	res := ParseCableDiag(port, raw)
	res.Commands = cmds
	return res, nil
}

var (
	cablePairRe  = regexp.MustCompile(`^(1-2|3-6|4-5|7-8)$`)
	cablePortRe  = regexp.MustCompile(`^(?:\d+/\d+/\d+|[A-Za-z]?\d+)$`)
	cableImpRe   = regexp.MustCompile(`^(?:\d+-\d+|[<>]=?\d+|--|n/?a)$`)
	cableNumDist = regexp.MustCompile(`^\d+(?:\.\d+)?(?:\s*\+/-\s*\d+(?:\.\d+)?)?$`)
)

func hasCablePairs(s string) bool {
	return strings.Contains(s, "1-2") && strings.Contains(s, "3-6")
}

func cableUnsupported(s string) bool {
	low := strings.ToLower(s)
	return strings.Contains(low, "not supported") ||
		strings.Contains(low, "fiber") ||
		strings.Contains(low, "only supported on copper") ||
		strings.Contains(low, "invalid input") ||
		strings.Contains(low, "cannot run cable")
}

// ParseCableDiag parses both Aruba AOS-CX and AOS-S TDR cable diagnostic outputs.
func ParseCableDiag(port, raw string) CableDiagResult {
	res := CableDiagResult{
		Port:      port,
		Supported: true,
		Health:    "unknown",
		Raw:       raw,
		TestedAt:  time.Now(),
	}
	if cableUnsupported(raw) {
		res.Supported = false
		res.Summary = "Port does not support copper TDR cable diagnostics (e.g. fiber/SFP port or unsupported switch)."
		return res
	}

	curPort := ""
	seen := map[string]int{}
	for _, line := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "---") || strings.HasPrefix(t, "> ") {
			continue
		}
		fs := strings.Fields(t)
		if len(fs) < 2 {
			continue
		}
		pairIdx := -1
		if cablePairRe.MatchString(fs[0]) {
			pairIdx = 0
		} else if len(fs) >= 3 && cablePortRe.MatchString(fs[0]) && cablePairRe.MatchString(fs[1]) {
			curPort = fs[0]
			pairIdx = 1
		} else if cablePortRe.MatchString(fs[0]) && strings.Contains(strings.ToLower(t), "not supported") {
			curPort = fs[0]
			continue
		} else {
			continue
		}
		if curPort != "" && !strings.EqualFold(curPort, port) {
			continue
		}

		pair := fs[pairIdx]
		status := fs[pairIdx+1]
		rest := fs[pairIdx+2:]

		mdi := ""
		if len(rest) > 0 {
			last := strings.ToLower(rest[len(rest)-1])
			if last == "mdi" || last == "mdix" || last == "mdi-x" || last == "auto" {
				mdi = rest[len(rest)-1]
				rest = rest[:len(rest)-1]
			}
		}

		imp := ""
		if len(rest) >= 2 && cableImpRe.MatchString(strings.ToLower(rest[0])) {
			imp = rest[0]
			rest = rest[1:]
		}

		dist := strings.TrimSpace(strings.Join(rest, " "))
		if dist != "" && cableNumDist.MatchString(dist) {
			dist += " m"
		}

		sl := strings.ToLower(status)
		ok := sl == "good" || sl == "ok" || sl == "normal"

		cp := CablePair{
			Pair:      pair,
			Status:    status,
			OK:        ok,
			Impedance: imp,
			Distance:  dist,
			MDIMode:   mdi,
		}
		if idx, exists := seen[pair]; exists {
			res.Pairs[idx] = cp
		} else {
			seen[pair] = len(res.Pairs)
			res.Pairs = append(res.Pairs, cp)
		}
	}

	if len(res.Pairs) == 0 {
		res.Summary = "No pair readings returned by the switch (check raw CLI output)."
		return res
	}

	goodCnt, openCnt, faultCnt := 0, 0, 0
	for _, p := range res.Pairs {
		sl := strings.ToLower(p.Status)
		switch {
		case p.OK:
			goodCnt++
		case strings.Contains(sl, "open"):
			openCnt++
		default:
			faultCnt++
		}
	}
	switch {
	case goodCnt == len(res.Pairs):
		res.Health = "ok"
		res.Summary = fmt.Sprintf("All %d pairs OK — cable is healthy.", len(res.Pairs))
	case faultCnt > 0:
		res.Health = "fault"
		res.Summary = "Cable fault detected (short circuit or impedance mismatch). Check RJ45 termination and cable run."
	case openCnt == len(res.Pairs):
		res.Health = "open"
		res.Summary = "All pairs Open — port is unplugged or cable is completely severed."
	default:
		res.Health = "fault"
		res.Summary = "Partial pair break detected (some pairs OK, others Open) — common cause of Gigabit dropping to 100 Mbps."
	}
	return res
}
