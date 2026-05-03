# Crabbox + static-ssh

> Fork of [openclaw/crabbox](https://github.com/openclaw/crabbox) that adds `static-ssh` — use any always-on Linux machine at home as your testbox. No cloud leasing needed.

**Turn your old laptop into a remote test runner.** Warm a box, sync the diff, run the suite — all on hardware you already own.

```sh
crabbox run -- pnpm test
# ^ runs on your home laptop, not the cloud
```

---

## What static-ssh adds

Normally Crabbox leases Hetzner or AWS machines. With `static-ssh`, you point it at any Linux machine reachable over SSH — an old laptop, a NUC, a Raspberry Pi, a home server. The machine stays on 24/7. Crabbox syncs your checkout over rsync and runs commands there.

| Feature | Cloud | static-ssh |
|---------|-------|------------|
| Provisioning | API-driven | Skipped (always on) |
| Sync + rsync | Yes | Yes |
| Fingerprint skip | Yes | Yes |
| Warmup / reuse | Yes | Yes |
| Cost | Hourly billing | Zero |
| Multi-user (coordinator) | Yes | Yes (if you deploy the worker) |
| Auto-expiry | Yes | Optional |

---

## Quick start

### 1. Prep your Linux box (one time)

```bash
# Install prerequisites
sudo apt-get install -y git rsync curl jq

# Create work root
sudo mkdir -p /work/crabbox
sudo chown $USER:$USER /work/crabbox

# Install crabbox-ready check
sudo tee /usr/local/bin/crabbox-ready > /dev/null <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
git --version && rsync --version >/dev/null && curl --version >/dev/null && jq --version >/dev/null
test -w /work/crabbox
EOF
sudo chmod +x /usr/local/bin/crabbox-ready

# Add your SSH key
ssh-copy-id user@your-box.local
```

### 2. Install Crabbox

```bash
# Build from source
git clone https://github.com/JithendraNara/crabbox
cd crabbox
go build -o /usr/local/bin/crabbox ./cmd/crabbox

# Verify
crabbox --version
```

### 3. Configure

`~/.config/crabbox/config.yaml`:

```yaml
provider: static-ssh
static:
  host: 192.168.0.193          # or your-box.local, tailscale hostname
ssh:
  user: jithendra
  key: ~/.ssh/id_ed25519
  port: "22"
workRoot: /home/jithendra/crabbox
```

Or use env vars: `CRABBOX_PROVIDER=static-ssh CRABBOX_STATIC_SSH_HOST=192.168.0.193`

### 4. Run

```bash
# Verify
crabbox doctor

# One-shot: sync, run, release
crabbox run -- pnpm test

# Warm + reuse
crabbox warmup                       # → cbx_... + slug
crabbox run --id blue-prawn -- pnpm test:changed
crabbox stop blue-prawn
```

Every lease gets a `cbx_...` ID and a crustacean slug (`blue-prawn`, `swift-lobster`, ...).

---

## Also works with clouds

The original Hetzner, AWS EC2 Spot, and Blacksmith Testbox providers still work. See [upstream docs](https://github.com/openclaw/crabbox) for those.

```yaml
# Switch to cloud when you need muscle
provider: aws
broker:
  url: https://crabbox.openclaw.ai
  provider: aws
```

---

## How it works

```
your laptop                                              your home linux box
-------------                                            ------------------
crabbox CLI    -- SSH + rsync  -->                       Ubuntu / Debian
   |                                                        |
   | (optional) HTTPS                                       |
   +----------------------->  Cloudflare Worker             |
                              lease + usage state
```

- **CLI** — Go binary. Syncs your dirty checkout with rsync (fingerprint skip when unchanged), runs commands over SSH, streams output.
- **Home box** — Any Linux machine with git, rsync, curl, jq, and OpenSSH. No cloud credentials on the box.
- **Coordinator** (optional) — Cloudflare Worker + Durable Object for multi-user lease management, usage tracking, and expiry.

---

## Why this fork exists

I had a perfectly good Linux laptop sitting at home doing nothing. It has an i5, 8GB RAM, 1TB disk, and stays online 24/7. This fork lets me use it as my Crabbox runner instead of paying for cloud instances.

---

## Development

```bash
# Go CLI
go build -o bin/crabbox ./cmd/crabbox
go test -race ./...
go vet ./...

# Cloudflare Worker (if deploying coordinator)
npm ci --prefix worker
npm test --prefix worker
npm run check --prefix worker
```

---

## License

MIT — see [LICENSE](LICENSE). Forked from [openclaw/crabbox](https://github.com/openclaw/crabbox) (MIT).
