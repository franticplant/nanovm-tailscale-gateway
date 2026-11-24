package forwarder

import (
	"context"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"
)

func TestAllows(t *testing.T) {
	f := NewTCP([]netip.Prefix{
		netip.MustParsePrefix("10.231.10.0/24"),
		netip.MustParsePrefix("10.231.20.0/24"),
	}, time.Second, 10, nil)

	tests := []struct {
		addr string
		want bool
	}{
		{"10.231.10.10", true},
		{"10.231.20.99", true},
		{"10.231.30.10", false},
		{"127.0.0.1", false},
		{"0.0.0.0", false},
		{"224.0.0.1", false},
	}

	for _, tc := range tests {
		got := f.Allows(netip.MustParseAddr(tc.addr))
		if got != tc.want {
			t.Errorf("Allows(%s) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}

func TestFallbackHandlerRejectsUnadvertised(t *testing.T) {
	f := NewTCP([]netip.Prefix{netip.MustParsePrefix("10.231.10.0/24")}, time.Second, 10, nil)
	h, intercept := f.FallbackHandler(
		netip.MustParseAddrPort("100.64.0.1:40000"),
		netip.MustParseAddrPort("192.168.1.1:22"),
	)
	if !intercept || h != nil {
		t.Fatalf("expected (nil, true) reject for unadvertised route, got (nonNil=%v, intercept=%v)", h != nil, intercept)
	}
	_, _, denied := f.Stats()
	if denied != 1 {
		t.Fatalf("denied=%d, want 1", denied)
	}
}

func TestFallbackHandlerProxiesEcho(t *testing.T) {
	// Start an in-process TCP echo server.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(conn, conn)
	}()

	f := NewTCP([]netip.Prefix{netip.MustParsePrefix("10.231.10.0/24")}, 2*time.Second, 4, nil)
	// Redirect dialer to our local echo server while testing the 10.231.10.10 allowlist.
	f.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, ln.Addr().String())
	}

	h, intercept := f.FallbackHandler(
		netip.MustParseAddrPort("100.64.0.1:40000"),
		netip.MustParseAddrPort("10.231.10.10:8080"),
	)
	if !intercept || h == nil {
		t.Fatalf("expected non-nil handler, got (nonNil=%v, intercept=%v)", h != nil, intercept)
	}

	c1, c2 := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		h(c2)
	}()

	msg := "hello-nanos-gateway"
	if _, err := c1.Write([]byte(msg)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, len(msg))
	if _, err := io.ReadFull(c1, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != msg {
		t.Fatalf("echo=%q, want %q", string(buf), msg)
	}
	_ = c1.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not exit after client closed")
	}

	active, total, denied := f.Stats()
	if active != 0 || total != 1 || denied != 0 {
		t.Fatalf("stats=(%d,%d,%d), want (0,1,0)", active, total, denied)
	}
}
