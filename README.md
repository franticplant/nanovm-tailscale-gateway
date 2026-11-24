# nanovm-tailscale-gateway

A minimal, immutable **Tailscale TCP subnet-routing appliance** built as a single statically compiled Go binary (`tailscale.com/tsnet`) running on the unmodified upstream **[Nanos](https://github.com/nanovms/nanos)** unikernel.

Designed for Proxmox VE / QEMU/KVM environments where you want a dedicated, shell-less, read-only Tailscale gateway into isolated management and guest VNets — with Tailscale identity persisted on a separate writable volume so the system image can be replaced atomically on every update.

## What it is (and is not)

- **Zero Linux userspace:** no shell, no SSH daemon, no package manager, no separate `tailscaled` daemon.
- **Unmodified upstream dependencies:** upstream Nanos kernel, upstream OPS, upstream Go, and upstream `tailscale.com/tsnet` (no forks or patches).
- **Immutable system disk + separate state volume:** `readonly_rootfs` system image with zero credentials baked in; Tailscale state lives on a dedicated `/data` VirtIO volume.
- **TCP-only subnet routing (by design):** uses `tsnet.Server.RegisterFallbackTCPHandler`, strictly scoped to explicitly advertised CIDRs (refuses `0.0.0.0/0`, `::/0`, loopback, and unadvertised destinations). Covers SSH, HTTPS APIs, WebSockets (e.g., Proxmox consoles), SFTP/SCP, and internal TCP services.
- **Not an exit node or UDP router:** `tsnet` does not expose a UDP fallback forwarding hook, and this appliance intentionally refuses default routes (`0.0.0.0/0` / `::/0`).
- **Source IP semantics:** userspace TCP proxying means destination services see the gateway's VNet IP, while end-to-end SSH/TLS encryption is preserved.

## Quick start

### Using local Go, OPS, and QEMU

Requires Go (matching `go.mod`), [NanoVMs OPS](https://ops.city/), and QEMU (`qemu-system-x86_64`, `qemu-img`).

```bash
# 1. Run tests
go vet ./...
go test -race ./...

# 2. Build static Linux amd64 binary
mkdir -p build
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o build/gateway ./cmd/gateway

# 3. Build a Nanos smoke image and boot it under QEMU
source config/versions.env
ops update --version "$NANOS_VERSION"
ops build build/gateway --nanos-version "$NANOS_VERSION" -c config/ops.smoke.json -i gateway-smoke
./scripts/smoke-qemu.sh ~/.ops/images/gateway-smoke
```

### Containerized / reproducible builds (`podman` or `docker`)

If you prefer not to install Go or OPS on your host, the `Makefile` runs both inside temporary containers (fetching into `~/.cache/` first, then building with `--network=none`):

```bash
make fetch
make vet test
make smoke
```

See [docs/compatibility.md](docs/compatibility.md) for Nanos/`tsnet` runtime findings and [docs/architecture.md](docs/architecture.md) for the storage, multi-NIC, and update model.

## CI/CD & Release Discipline

- **PR CI (`ci.yml`):** `go vet`, `go test -race`, `govulncheck`, `gitleaks`, static amd64 build, pinned Nanos image build, and a real QEMU boot smoke test (`SMOKE_TEST_OK` + multi-NIC check).
- **Dependabot (`dependabot.yml` + `dependabot-automerge.yml`):** automated daily `gomod` and weekly `github-actions` updates; patch/minor updates auto-merge only when all CI checks pass.
- **Nanos updates (`check-nanos-updates.yml`):** daily check for new upstream Nanos releases, opening a PR gated by CI + QEMU smoke test.
- **Daily & tagged builds (`daily-build.yml`, `release.yml`):** publishes `.img`, `.qcow2`, `SHA256SUMS`, `build-info.json`, and GitHub build provenance attestations.
