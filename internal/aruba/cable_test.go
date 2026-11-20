package aruba

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestParseCableDiagAOSCX(t *testing.T) {
	healthy := `
Cable Impedance Distance* MDI Interface Pinout Status (Ohms) (Meters) Mode
--------------------------------------------------------------------
1/3/1 1-2 good 85-115 10 mdi
      3-6 good 85-115 10 mdi
      4-5 good 85-115 10 mdi
      7-8 good 85-115 10 mdi
`
	res := ParseCableDiag("1/3/1", healthy)
	if !res.Supported || res.Health != "ok" || len(res.Pairs) != 4 {
		t.Fatalf("healthy CX = %+v", res)
	}
	if res.Pairs[0].Distance != "10 m" || res.Pairs[0].Impedance != "85-115" || res.Pairs[0].MDIMode != "mdi" {
		t.Fatalf("pair 0 = %+v", res.Pairs[0])
	}

	fault := `
Interface Pinout Status Ohms Meters Mode
--------------------------------------------------------------------
1/1/20 1-2 good 85-115 -- mdi
       3-6 open >300 4 +/- 5 mdi
       4-5 open >300 4 +/- 5 mdi
       7-8 intra-short <30 3 +/- 5 mdi
`
	fres := ParseCableDiag("1/1/20", fault)
	if fres.Health != "fault" || len(fres.Pairs) != 4 || !fres.Pairs[0].OK || fres.Pairs[3].OK {
		t.Fatalf("fault CX = %+v", fres)
	}
	if fres.Pairs[1].Distance != "4 +/- 5 m" || fres.Pairs[3].Status != "intra-short" {
		t.Fatalf("fault pairs = %+v", fres.Pairs)
	}
}

func TestParseCableDiagAOSS(t *testing.T) {
	out := `
Cable Diagnostic Status - Copper Ports

Port  Pair   Status   Cable Length or Distance to Fault
----  -----  -------  ---------------------------------
1     1-2    OK       2m
      3-6    OK       4m
      4-5    OK       2m
      7-8    OK       2m

2     1-2    Open     0m
      3-6    Open     0m
      4-5    Open     0m
      7-8    Open     0m

3     1-2    OK       31m
      3-6    OK       36m
      4-5    Open     18m
      7-8    Open     18m
`
	r1 := ParseCableDiag("1", out)
	if r1.Health != "ok" || len(r1.Pairs) != 4 || r1.Pairs[1].Distance != "4m" {
		t.Fatalf("port 1 = %+v", r1)
	}
	r2 := ParseCableDiag("2", out)
	if r2.Health != "open" || len(r2.Pairs) != 4 {
		t.Fatalf("port 2 = %+v", r2)
	}
	r3 := ParseCableDiag("3", out)
	if r3.Health != "fault" || !strings.Contains(r3.Summary, "100 Mbps") {
		t.Fatalf("port 3 partial break = %+v", r3)
	}
}

type fakeCableRunner struct {
	calls []string
	reply func(cmd string, n int) string
}

func (f *fakeCableRunner) Run(_ context.Context, cmd string, _ time.Duration) (string, error) {
	f.calls = append(f.calls, cmd)
	return f.reply(cmd, len(f.calls)), nil
}

func TestRunCableDiagFlow(t *testing.T) {
	oldWait := cableWait
	cableWait = time.Millisecond
	defer func() { cableWait = oldWait }()

	cx := &fakeCableRunner{reply: func(cmd string, n int) string {
		if strings.HasPrefix(cmd, "diag cable-diagnostic test") {
			return "Continue (y/n)? y\n"
		}
		if n == 2 {
			return "1/1/5 1-2 in-progress -- -- mdi\n"
		}
		return "1/1/5 1-2 good 85-115 12 mdi\n 3-6 good 85-115 12 mdi\n 4-5 good 85-115 12 mdi\n 7-8 good 85-115 12 mdi\n"
	}}
	res, err := RunCableDiag(context.Background(), cx, FamilyAOSCX, "1/1/5")
	if err != nil || res.Health != "ok" || len(res.Pairs) != 4 || len(cx.calls) != 3 {
		t.Fatalf("CX flow: err=%v calls=%v res=%+v", err, cx.calls, res)
	}

	s := &fakeCableRunner{reply: func(cmd string, _ int) string {
		if strings.HasPrefix(cmd, "test cable-diagnostics") {
			return "Continue (y/n)? y\n"
		}
		return "5 1-2 OK 15m\n 3-6 OK 15m\n 4-5 Short 9m\n 7-8 OK 15m\n"
	}}
	sres, err := RunCableDiag(context.Background(), s, FamilyAOSS, "5")
	if err != nil || sres.Health != "fault" || len(sres.Pairs) != 4 {
		t.Fatalf("AOS-S flow: err=%v calls=%v res=%+v", err, s.calls, sres)
	}
}
