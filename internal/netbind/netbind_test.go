package netbind

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestExpandScanTarget(t *testing.T) {
	ips, err := ExpandScanTarget("10.0.0.0/24")
	if err != nil || len(ips) != 254 || ips[0] != "10.0.0.1" || ips[253] != "10.0.0.254" {
		t.Fatalf("unexpected /24 expansion: err=%v len=%d first=%v", err, len(ips), ips)
	}
	rng, err := ExpandScanTarget("10.0.0.10 - 10.0.0.15")
	if err != nil || len(rng) != 6 || rng[0] != "10.0.0.10" || rng[5] != "10.0.0.15" {
		t.Fatalf("unexpected range expansion: err=%v got=%v", err, rng)
	}
	shortRng, err := ExpandScanTarget("10.0.0.20-22")
	if err != nil || len(shortRng) != 3 || shortRng[2] != "10.0.0.22" {
		t.Fatalf("unexpected short range expansion: err=%v got=%v", err, shortRng)
	}
}

func TestBindIPAndScanLocalListener(t *testing.T) {
	SetBindIP("10.255.255.254")
	if got := GetBindIP(); got != "10.255.255.254" {
		t.Fatalf("GetBindIP() = %q, want 10.255.255.254", got)
	}
	SetBindIP("")
	if got := GetBindIP(); got != "" {
		t.Fatalf("GetBindIP() after reset = %q, want empty", got)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_, _ = fmt.Fprintln(c, "SSH-2.0-OpenSSH_8.9")
			_ = c.Close()
		}
	}()

	conn, err := DialContext(context.Background(), "tcp", ln.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("DialContext failed: %v", err)
	}
	_ = conn.Close()
}
