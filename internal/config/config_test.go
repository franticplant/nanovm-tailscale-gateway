package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseRoutes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "empty", input: "", want: 0},
		{name: "single normalized", input: "10.231.10.5/24", want: 1},
		{name: "multiple with dedup", input: "10.231.10.0/24, 10.231.20.0/24, 10.231.10.1/24", want: 2},
		{name: "reject ipv4 default route", input: "0.0.0.0/0", wantErr: true},
		{name: "reject ipv6 default route", input: "::/0", wantErr: true},
		{name: "reject loopback", input: "127.0.0.0/8", wantErr: true},
		{name: "invalid cidr", input: "not-a-cidr", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseRoutes(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseRoutes(%q) expected error, got %v", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRoutes(%q) unexpected error: %v", tc.input, err)
			}
			if len(got) != tc.want {
				t.Fatalf("ParseRoutes(%q) len=%d, want %d (%v)", tc.input, len(got), tc.want, got)
			}
		})
	}
}

func TestLoadJSONAndValidate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appliance.json")
	content := `{
		"hostname": "gw-prod",
		"state_dir": "/data/ts",
		"routes": ["10.231.10.0/24", "10.231.20.0/24"],
		"dial_timeout": "5s",
		"max_concurrent": 128
	}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	if err := LoadJSON(path, &cfg); err != nil {
		t.Fatalf("LoadJSON failed: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if cfg.Hostname != "gw-prod" {
		t.Fatalf("Hostname=%q, want gw-prod", cfg.Hostname)
	}
	if cfg.DialTimeout != 5*time.Second {
		t.Fatalf("DialTimeout=%v, want 5s", cfg.DialTimeout)
	}
	if cfg.MaxConcurrent != 128 {
		t.Fatalf("MaxConcurrent=%d, want 128", cfg.MaxConcurrent)
	}
	if len(cfg.Routes) != 2 {
		t.Fatalf("Routes=%v, want 2 routes", cfg.Routes)
	}
}

func TestResolveAuthKeyFromFile(t *testing.T) {
	t.Setenv("TS_AUTHKEY", "")
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key")
	if err := os.WriteFile(keyPath, []byte("tskey-auth-example\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	cfg.AuthKeyFile = keyPath
	if err := cfg.ResolveAuthKey(); err != nil {
		t.Fatalf("ResolveAuthKey failed: %v", err)
	}
	if cfg.AuthKey != "tskey-auth-example" {
		t.Fatalf("AuthKey=%q, want tskey-auth-example", cfg.AuthKey)
	}
}
