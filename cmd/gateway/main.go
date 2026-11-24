package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/franticplant/nanovm-tailscale-gateway/internal/config"
	"github.com/franticplant/nanovm-tailscale-gateway/internal/forwarder"
	"tailscale.com/ipn"
	"tailscale.com/tsnet"
)

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func logInterfaces() {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Printf("net.Interfaces() error: %v", err)
		return
	}
	log.Printf("discovered %d network interface(s):", len(ifaces))
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			log.Printf("  [%d] %s flags=%v addrs_err=%v", iface.Index, iface.Name, iface.Flags, err)
			continue
		}
		var addrStrs []string
		for _, a := range addrs {
			addrStrs = append(addrStrs, a.String())
		}
		log.Printf("  [%d] %s flags=%v addrs=%v", iface.Index, iface.Name, iface.Flags, addrStrs)
	}
}

func runSmoke(cfg config.Config, fwd *forwarder.TCP) {
	if err := os.MkdirAll(cfg.StateDir, 0700); err != nil {
		log.Fatalf("smoke: creating state dir %q: %v", cfg.StateDir, err)
	}
	probe := filepath.Join(cfg.StateDir, ".smoke-probe")
	if err := os.WriteFile(probe, []byte("ok\n"), 0600); err != nil {
		log.Fatalf("smoke: writing state probe %q: %v", probe, err)
	}
	_ = os.Remove(probe)

	// Verify fail-closed allowlist behavior in forwarder.
	if len(cfg.Routes) > 0 && !fwd.Allows(cfg.Routes[0].Addr()) {
		log.Fatalf("smoke: expected route %s to be allowed", cfg.Routes[0])
	}
	log.Printf("SMOKE_TEST_OK hostname=%q stateDir=%q routes=%v", cfg.Hostname, cfg.StateDir, cfg.Routes)
}

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC | log.Lmicroseconds)

	cfg := config.DefaultConfig()

	configFile := flag.String("config", envOr("GATEWAY_CONFIG", ""), "Optional JSON config file path")
	hostname := flag.String("hostname", envOr("GATEWAY_HOSTNAME", cfg.Hostname), "Tailscale hostname")
	stateDir := flag.String("state-dir", envOr("GATEWAY_STATE_DIR", cfg.StateDir), "Tailscale state directory")
	routesCSV := flag.String("routes", envOr("GATEWAY_ROUTES", ""), "Comma-separated CIDR routes to advertise")
	authKeyFile := flag.String("authkey-file", envOr("GATEWAY_AUTHKEY_FILE", ""), "Optional path to file containing Tailscale auth key")
	ephemeral := flag.Bool("ephemeral", envOr("GATEWAY_EPHEMERAL", "false") == "true", "Register as ephemeral Tailscale node")
	verbose := flag.Bool("verbose", envOr("GATEWAY_VERBOSE", "false") == "true", "Enable verbose tsnet backend logging")
	smoke := flag.Bool("smoke", envOr("GATEWAY_SMOKE", "false") == "true", "Run offline boot smoke self-test and exit")
	dialTimeout := flag.Duration("dial-timeout", cfg.DialTimeout, "TCP dial timeout for forwarded connections")
	maxConcurrent := flag.Int("max-concurrent", cfg.MaxConcurrent, "Maximum concurrent forwarded TCP connections")
	flag.Parse()

	if *configFile != "" {
		if err := config.LoadJSON(*configFile, &cfg); err != nil {
			log.Fatalf("config error: %v", err)
		}
	}

	// Explicit flags/env override defaults (and JSON when provided).
	cfg.Hostname = *hostname
	cfg.StateDir = *stateDir
	if *routesCSV != "" {
		routes, err := config.ParseRoutes(*routesCSV)
		if err != nil {
			log.Fatalf("route config error: %v", err)
		}
		cfg.Routes = routes
	}
	if *authKeyFile != "" {
		cfg.AuthKeyFile = *authKeyFile
	}
	cfg.Ephemeral = *ephemeral
	cfg.Verbose = *verbose
	cfg.Smoke = *smoke
	cfg.DialTimeout = *dialTimeout
	cfg.MaxConcurrent = *maxConcurrent

	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	log.Printf("gateway starting (go=%s os=%s arch=%s pid=%d)", runtime.Version(), runtime.GOOS, runtime.GOARCH, os.Getpid())
	logInterfaces()

	fwd := forwarder.NewTCP(cfg.Routes, cfg.DialTimeout, cfg.MaxConcurrent, log.Printf)

	if cfg.Smoke {
		runSmoke(cfg, fwd)
		return
	}

	if err := cfg.ResolveAuthKey(); err != nil {
		log.Fatalf("authkey error: %v", err)
	}
	if err := os.MkdirAll(cfg.StateDir, 0700); err != nil {
		log.Fatalf("creating state dir %q: %v", cfg.StateDir, err)
	}

	srv := &tsnet.Server{
		Hostname:  cfg.Hostname,
		Dir:       cfg.StateDir,
		AuthKey:   cfg.AuthKey,
		Ephemeral: cfg.Ephemeral,
		UserLogf:  log.Printf,
	}
	if cfg.Verbose {
		srv.Logf = log.Printf
	}
	defer srv.Close()

	if len(cfg.Routes) > 0 {
		srv.RegisterFallbackTCPHandler(fwd.FallbackHandler)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("starting tsnet server hostname=%q stateDir=%q ephemeral=%v routes=%v",
		cfg.Hostname, cfg.StateDir, cfg.Ephemeral, cfg.Routes)

	lc, err := srv.LocalClient()
	if err != nil {
		log.Fatalf("tsnet.LocalClient failed: %v", err)
	}

	mp := &ipn.MaskedPrefs{
		Prefs: ipn.Prefs{
			AdvertiseRoutes: cfg.Routes,
		},
		AdvertiseRoutesSet: true,
	}
	if _, err := lc.EditPrefs(ctx, mp); err != nil {
		log.Fatalf("EditPrefs(AdvertiseRoutes=%v) failed: %v", cfg.Routes, err)
	}
	log.Printf("configured AdvertiseRoutes=%v", cfg.Routes)

	upCtx, cancelUp := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelUp()

	status, err := srv.Up(upCtx)
	if err != nil {
		log.Fatalf("tsnet.Up failed: %v", err)
	}

	log.Printf("tsnet is UP! BackendState=%s TailscaleIPs=%v Self=%s",
		status.BackendState, status.TailscaleIPs, status.Self.DNSName)
	if prefs, err := lc.GetPrefs(ctx); err == nil {
		log.Printf("active prefs: Hostname=%q AdvertiseRoutes=%v", prefs.Hostname, prefs.AdvertiseRoutes)
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("shutdown signal received; closing tsnet server")
			return
		case <-ticker.C:
			st, err := lc.StatusWithoutPeers(ctx)
			if err != nil {
				log.Printf("status check error: %v", err)
				continue
			}
			active, total, denied := fwd.Stats()
			log.Printf("health: BackendState=%s TailscaleIPs=%v tcp_active=%d tcp_total=%d tcp_denied=%d Health=%v",
				st.BackendState, st.TailscaleIPs, active, total, denied, st.Health)
		}
	}
}
