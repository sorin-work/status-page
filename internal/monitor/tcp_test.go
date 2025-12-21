package monitor

import (
	"context"
	"net"
	"testing"
	"time"
)

func tcpMonitor(addr string) Monitor {
	return Monitor{ID: 3, Name: "tcp", Type: TypeTCP, Target: addr, Interval: time.Minute, Timeout: time.Second}
}

func TestTCPCheckUp(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	res := NewChecker().Check(context.Background(), tcpMonitor(ln.Addr().String()))
	if !res.Up || res.Error != "" {
		t.Fatalf("expected up: %+v", res)
	}
	if res.StatusCode != 0 {
		t.Errorf("TCP checks have no status code, got %d", res.StatusCode)
	}
}

func TestTCPCheckRefused(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	res := NewChecker().Check(context.Background(), tcpMonitor(addr))
	if res.Up || res.Error == "" {
		t.Fatalf("expected failure: %+v", res)
	}
}

func TestUnsupportedType(t *testing.T) {
	res := NewChecker().Check(context.Background(), Monitor{ID: 1, Type: "icmp"})
	if res.Up || res.Error == "" {
		t.Fatalf("expected failure: %+v", res)
	}
}
