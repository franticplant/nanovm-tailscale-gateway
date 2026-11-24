# Security Policy

## Scope and threat model notes

- A minimal unikernel reduces traditional Linux userspace attack surface, **not** the privilege of a network gateway.
- Always enforce external hypervisor/Proxmox firewall rules restricting which VNet destinations the gateway VM can reach.
- Never bake reusable Tailscale auth keys into replaceable system images. Provision identity once onto the dedicated persistent state volume (`/data`), then run credential-free read-only system images (`readonly_rootfs: "t"` + hypervisor `readonly=on`).
- Never attach the same Tailscale state volume to two running VM instances simultaneously.
- A read-only system image does not by itself provide verified boot or cryptographic image integrity; verify `SHA256SUMS` and GitHub build provenance attestations before importing an image.

## Reporting a vulnerability

Please report security issues privately via GitHub Security Advisories for this repository rather than opening a public issue.
