package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"time"
)

// Config holds the runtime configuration for the Nanos tsnet TCP subnet gateway.
type Config struct {
	Hostname      string         `json:"hostname"`
	StateDir      string         `json:"state_dir"`
	Routes        []netip.Prefix `json:"routes"`
	AuthKey       string         `json:"-"`
	AuthKeyFile   string         `json:"authkey_file,omitempty"`
	Ephemeral     bool           `json:"ephemeral"`
	Verbose       bool           `json:"verbose"`
	Smoke         bool           `json:"smoke,omitempty"`
	DialTimeout   time.Duration  `json:"dial_timeout"`
	MaxConcurrent int            `json:"max_concurrent"`
}

// DefaultConfig returns safe defaults for the appliance.
func DefaultConfig() Config {
	return Config{
		Hostname:      "nanos-tsnet-gateway",
		StateDir:      "/data/tailscale",
		DialTimeout:   10 * time.Second,
		MaxConcurrent: 256,
	}
}

// ParseRoutes parses a comma-separated list of CIDR prefixes and normalizes them.
func ParseRoutes(csv string) ([]netip.Prefix, error) {
	csv = strings.TrimSpace(csv)
	if csv == "" {
		return nil, nil
	}
	var out []netip.Prefix
	seen := make(map[netip.Prefix]struct{})
	for _, raw := range strings.Split(csv, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		pfx, err := netip.ParsePrefix(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid route %q: %w", raw, err)
		}
		pfx = pfx.Masked()
		if pfx.Bits() == 0 {
			return nil, fmt.Errorf("refusing default/exit route %q: appliance is not an exit node", pfx)
		}
		if pfx.Addr().IsLoopback() {
			return nil, fmt.Errorf("refusing loopback route %q", pfx)
		}
		if _, dup := seen[pfx]; dup {
			continue
		}
		seen[pfx] = struct{}{}
		out = append(out, pfx)
	}
	return out, nil
}

// LoadJSON overlays settings from a JSON config file onto cfg.
func LoadJSON(path string, cfg *Config) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config file %q: %w", path, err)
	}
	var raw struct {
		Hostname      string   `json:"hostname"`
		StateDir      string   `json:"state_dir"`
		Routes        []string `json:"routes"`
		AuthKeyFile   string   `json:"authkey_file"`
		Ephemeral     *bool    `json:"ephemeral"`
		Verbose       *bool    `json:"verbose"`
		DialTimeout   string   `json:"dial_timeout"`
		MaxConcurrent *int     `json:"max_concurrent"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("parsing config file %q: %w", path, err)
	}
	if raw.Hostname != "" {
		cfg.Hostname = raw.Hostname
	}
	if raw.StateDir != "" {
		cfg.StateDir = raw.StateDir
	}
	if len(raw.Routes) > 0 {
		routes, err := ParseRoutes(strings.Join(raw.Routes, ","))
		if err != nil {
			return err
		}
		cfg.Routes = routes
	}
	if raw.AuthKeyFile != "" {
		cfg.AuthKeyFile = raw.AuthKeyFile
	}
	if raw.Ephemeral != nil {
		cfg.Ephemeral = *raw.Ephemeral
	}
	if raw.Verbose != nil {
		cfg.Verbose = *raw.Verbose
	}
	if raw.DialTimeout != "" {
		d, err := time.ParseDuration(raw.DialTimeout)
		if err != nil {
			return fmt.Errorf("invalid dial_timeout %q: %w", raw.DialTimeout, err)
		}
		cfg.DialTimeout = d
	}
	if raw.MaxConcurrent != nil {
		cfg.MaxConcurrent = *raw.MaxConcurrent
	}
	return nil
}

// ResolveAuthKey loads the Tailscale auth key from TS_AUTHKEY or AuthKeyFile if present.
// A missing AuthKeyFile is ignored so reboots succeed after a one-shot authkey file is removed.
func (c *Config) ResolveAuthKey() error {
	if c.AuthKey != "" {
		return nil
	}
	if env := strings.TrimSpace(os.Getenv("TS_AUTHKEY")); env != "" {
		c.AuthKey = env
		return nil
	}
	if c.AuthKeyFile != "" {
		b, err := os.ReadFile(c.AuthKeyFile)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("reading authkey file %q: %w", c.AuthKeyFile, err)
		}
		c.AuthKey = strings.TrimSpace(string(b))
	}
	return nil
}

// Validate checks configuration invariants before startup.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Hostname) == "" {
		return errors.New("hostname must not be empty")
	}
	if strings.TrimSpace(c.StateDir) == "" {
		return errors.New("state_dir must not be empty")
	}
	if c.DialTimeout <= 0 {
		return fmt.Errorf("dial_timeout must be positive, got %v", c.DialTimeout)
	}
	if c.MaxConcurrent <= 0 {
		return fmt.Errorf("max_concurrent must be positive, got %d", c.MaxConcurrent)
	}
	for _, r := range c.Routes {
		if !r.IsValid() {
			return fmt.Errorf("invalid route prefix: %v", r)
		}
		if r.Bits() == 0 {
			return fmt.Errorf("refusing default/exit route %q", r)
		}
		if r.Addr().IsLoopback() {
			return fmt.Errorf("refusing loopback route %q", r)
		}
	}
	return nil
}
