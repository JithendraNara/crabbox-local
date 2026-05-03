# Providers

Read when:

- changing Hetzner, AWS, static-ssh, or Blacksmith Testbox provisioning;
- adding a backend;
- adjusting machine classes, fallback order, regions, or images.

Crabbox currently supports three brokered providers plus a static direct-only provider:

```text
hetzner     Hetzner Cloud servers
aws         AWS EC2 one-time Spot instances
static-ssh  Pre-existing machines reached directly over SSH (direct mode only)
```

Hetzner behavior:

- imports or reuses the lease SSH key;
- creates a server with Crabbox labels;
- uses configured image and location;
- falls back across class server types when capacity or quota rejects a request;
- fetches server-type hourly prices when cost estimates need provider pricing.

AWS behavior:

- signs EC2 Query API calls inside the Worker;
- imports or reuses an EC2 key pair;
- creates or reuses the `crabbox-runners` security group with SSH ingress limited to configured CIDRs or the request source IP;
- launches one-time Spot instances;
- tags instances, volumes, and Spot requests;
- falls back across broad C/M/R instance families for class requests, including account policy and capacity rejections;
- can fall back to a small burstable type when account policy rejects the high-core class candidates;
- preflights applied Spot or On-Demand vCPU quotas in brokered mode when Service Quotas allows it, then records skipped candidates as quota attempts;
- supports `--market spot|on-demand` on `warmup` and `run` for one-off capacity-market overrides;
- uses Spot placement score across configured regions in direct AWS mode;
- can fall back to On-Demand after Spot capacity/quota failures when configured;
- fetches Spot price history when cost estimates need provider pricing.

Explicit `--type` requests are treated as exact provider type requests. If that type is rejected, Crabbox fails clearly instead of silently choosing a different instance type. Remove `--type` and use a machine class when fallback is desired.

`crabbox list` marks brokered provider machines as `orphan=no-active-lease`
when their provider label references a lease that is no longer active in the
coordinator. This is an operator hint only; `keep=true` machines are not
deleted automatically.

Machine classes map to provider-specific types:

```text
Hetzner
standard  ccx33, cpx62, cx53
fast      ccx43, cpx62, cx53
large     ccx53, ccx43, cpx62, cx53
beast     ccx63, ccx53, ccx43, cpx62, cx53

AWS
standard  c7a.8xlarge, c7i.8xlarge, m7a.8xlarge, m7i.8xlarge, c7a.4xlarge
fast      c7a.16xlarge, c7i.16xlarge, m7a.16xlarge, m7i.16xlarge, c7a.12xlarge, c7a.8xlarge
large     c7a.24xlarge, c7i.24xlarge, m7a.24xlarge, m7i.24xlarge, r7a.24xlarge, c7a.16xlarge, c7a.12xlarge
beast     c7a.48xlarge, c7i.48xlarge, m7a.48xlarge, m7i.48xlarge, r7a.48xlarge, c7a.32xlarge, c7i.32xlarge, m7a.32xlarge, c7a.24xlarge, c7a.16xlarge
```

Direct provider mode still exists when no coordinator is configured. It uses local AWS credentials or `HCLOUD_TOKEN`/`HETZNER_TOKEN` and should stay secondary to the brokered path.

Static SSH behavior:

- no cloud API calls; connects directly to a pre-configured host over SSH;
- uses the configured `ssh.key` for authentication (no per-lease key generation);
- syncs via rsync and runs commands identically to brokered leases;
- local claims track leases across sessions;
- no cost tracking, no auto-expiry, no heartbeat — the machine is assumed to be always available;
- set `provider: static-ssh` + `static.host: <hostname>` in config, or use `CRABBOX_STATIC_SSH_HOST`.

Direct smoke shape:

```sh
tmp="$(mktemp)"
printf 'provider: hetzner\n' > "$tmp"
CRABBOX_CONFIG="$tmp" CRABBOX_COORDINATOR= crabbox warmup --provider hetzner --class standard --ttl 15m --idle-timeout 4m
CRABBOX_CONFIG="$tmp" CRABBOX_COORDINATOR= crabbox run --provider hetzner --id <slug> --no-sync -- echo direct-hetzner-ok
CRABBOX_CONFIG="$tmp" CRABBOX_COORDINATOR= crabbox stop --provider hetzner <slug>
rm -f "$tmp"
```

Use `--provider aws` with AWS SDK credentials for direct AWS smoke. Direct mode
has no Durable Object alarm; cleanup is best-effort through provider labels and
manual `crabbox cleanup`. Direct AWS fallback can retry provider types, but the
structured quota preflight and `provisioningAttempts` metadata belong to the
brokered Worker path.

Crabbox can also wrap Blacksmith Testboxes with `provider: blacksmith-testbox`. That backend does not use the Crabbox broker or direct cloud credentials. It shells out to the authenticated Blacksmith CLI for `testbox warmup`, `run`, `status`, `list`, and `stop`, while Crabbox keeps local slugs, repo claims, config, and timing summaries. See [Blacksmith Testbox](blacksmith-testbox.md).

Related docs:

- [Infrastructure](../infrastructure.md)
- [Blacksmith Testbox](blacksmith-testbox.md)
- [Runner bootstrap](runner-bootstrap.md)
- [Cost and usage](cost-usage.md)
