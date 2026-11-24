package forwarder

import (
	"context"
	"io"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"
)

// DialFunc dials a destination address (defaults to net.Dialer.DialContext).
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// TCP forwards incoming tsnet TCP flows whose destination is inside AllowedRoutes.
// Flows outside AllowedRoutes (or loopback/unspecified addresses) are rejected fail-closed.
type TCP struct {
	AllowedRoutes []netip.Prefix
	DialTimeout   time.Duration
	MaxConcurrent int
	DialContext   DialFunc
	Logf          func(format string, args ...any)

	sem    chan struct{}
	active atomic.Int64
	total  atomic.Uint64
	denied atomic.Uint64
}

// NewTCP creates a fail-closed TCP forwarder bounded by maxConcurrent active sessions.
func NewTCP(routes []netip.Prefix, dialTimeout time.Duration, maxConcurrent int, logf func(string, ...any)) *TCP {
	if dialTimeout <= 0 {
		dialTimeout = 10 * time.Second
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 256
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	d := &net.Dialer{Timeout: dialTimeout}
	cp := make([]netip.Prefix, len(routes))
	copy(cp, routes)
	return &TCP{
		AllowedRoutes: cp,
		DialTimeout:   dialTimeout,
		MaxConcurrent: maxConcurrent,
		DialContext:   d.DialContext,
		Logf:          logf,
		sem:           make(chan struct{}, maxConcurrent),
	}
}

// Allows reports whether dst is permitted by AllowedRoutes and is not loopback/unspecified.
func (f *TCP) Allows(dst netip.Addr) bool {
	if !dst.IsValid() || dst.IsLoopback() || dst.IsUnspecified() || dst.IsMulticast() {
		return false
	}
	for _, pfx := range f.AllowedRoutes {
		if pfx.Contains(dst) {
			return true
		}
	}
	return false
}

// Stats returns current active connections, total accepted, and total denied.
func (f *TCP) Stats() (active int64, total, denied uint64) {
	return f.active.Load(), f.total.Load(), f.denied.Load()
}

// FallbackHandler matches tsnet.FallbackTCPHandler.
func (f *TCP) FallbackHandler(src, dst netip.AddrPort) (func(net.Conn), bool) {
	if !f.Allows(dst.Addr()) {
		f.denied.Add(1)
		f.Logf("tcp forward deny: %s -> %s (not in allowed routes)", src, dst)
		return nil, true
	}

	return func(clientConn net.Conn) {
		defer clientConn.Close()

		select {
		case f.sem <- struct{}{}:
			defer func() { <-f.sem }()
		default:
			f.denied.Add(1)
			f.Logf("tcp forward reject: %s -> %s (max concurrent %d reached)", src, dst, f.MaxConcurrent)
			return
		}

		f.active.Add(1)
		defer f.active.Add(-1)
		f.total.Add(1)

		ctx, cancel := context.WithTimeout(context.Background(), f.DialTimeout)
		targetConn, err := f.DialContext(ctx, "tcp", dst.String())
		cancel()
		if err != nil {
			f.Logf("tcp forward dial error %s -> %s: %v", src, dst, err)
			return
		}
		defer targetConn.Close()

		f.Logf("tcp forward established %s -> %s (local=%s)", src, dst, targetConn.LocalAddr())
		proxyBidirectional(clientConn, targetConn)
	}, true
}

func proxyBidirectional(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	copyHalf := func(dst, src net.Conn) {
		defer wg.Done()
		_, _ = io.Copy(dst, src)
		if cw, ok := dst.(interface{ CloseWrite() error }); ok {
			_ = cw.CloseWrite()
		}
	}

	go copyHalf(b, a)
	go copyHalf(a, b)
	wg.Wait()
}
