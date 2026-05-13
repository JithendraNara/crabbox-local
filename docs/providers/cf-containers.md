# CF Containers Provider

Use `provider: cf-containers` when Crabbox should run commands through a
Cloudflare Worker backed by a custom Cloudflare Containers image. The provider
also accepts the aliases `cloudflare-containers`, `cloudflare`,
`cloudflare-sandbox`, and `cf-sandbox`.

CF Containers is a delegated run provider. Crabbox owns local repo archive
creation, local lease claims, timing output, command rendering, and friendly
slugs. A small Worker runner owns container creation, file upload, command
execution, and teardown.

## Requirements

- A Cloudflare account with Workers, Durable Objects, and Containers access.
- Wrangler authenticated for deploys.
- Docker or a Docker-compatible CLI/daemon available to Wrangler for container
  image builds.
- A deployed Crabbox CF Containers runner with `CRABBOX_RUNNER_TOKEN` set
  as a Worker secret.

The Worker coordinator lives in `worker/src/cloudflare_sandbox_runner.ts`. The
container image is built from `worker/cloudflare-sandbox.Dockerfile` and starts
the HTTP runner in `worker/cloudflare-container-runner`. The deploy config is
`worker/wrangler.cloudflare-sandbox.jsonc`.

## Configuration

```yaml
provider: cf-containers
cfContainers:
  apiUrl: https://crabbox-cloudflare-sandbox-runner.example.workers.dev
  workdir: /workspace/crabbox
```

Keep the bearer token in `CRABBOX_CF_CONTAINERS_TOKEN` or user-level config,
not in repo YAML. `CRABBOX_CF_CONTAINERS_URL` or
`CRABBOX_CF_CONTAINERS_API_URL` can also provide the runner URL. Legacy
`cloudflareSandbox` config and `CRABBOX_CLOUDFLARE_SANDBOX_*` environment
variables are still accepted for existing users.

Equivalent flags:

```sh
crabbox run \
  --provider cf-containers \
  --cf-containers-url https://runner.example.workers.dev \
  --cf-containers-token "$CRABBOX_CF_CONTAINERS_TOKEN" \
  -- pnpm test
```

## Runner Deploy

Install Worker dependencies and verify the runner:

```sh
npm ci --prefix worker
npm run check:cf-containers --prefix worker
npm run build:cf-containers --prefix worker
```

Deploy with:

```sh
npm run deploy:cf-containers --prefix worker
```

Then set the bearer token:

```sh
printf '%s' "$CRABBOX_CF_CONTAINERS_TOKEN" \
  | npx wrangler secret put CRABBOX_RUNNER_TOKEN \
      --config worker/wrangler.cloudflare-sandbox.jsonc
```

## Behavior

- `run` creates or reuses a Container Durable Object, uploads a gzipped archive
  of the local checkout, extracts it into `workdir`, and relays command output
  and exit status back to the CLI.
- `warmup` creates the container and prepares the workdir. Warmed containers
  remain alive until `crabbox stop` or the configured TTL/idle deadline
  expires.
- `status` and `stop` resolve Crabbox's local claim and call the runner.
- `list` reports local CF Containers claims. Cloudflare does not expose a
  global container listing API through the runner.
- The container image includes common repo-test tools such as Git, GitHub CLI,
  `jq`, `ripgrep`, Node, and `pnpm`; project-specific dependencies still belong
  to the repo's own setup commands.
- Warmed containers keep their container filesystem between commands while the
  lease is active. Use that as the provider's cache layer for cloned
  repositories, package stores, and generated setup state.
- The runner stores lease metadata in the Container Durable Object and schedules
  cleanup at the earlier of `--ttl` or `--idle-timeout`. Activity on file upload
  or command execution extends the idle deadline. When the deadline passes, the
  runner destroys the container and marks the lease expired.
- `crabbox cleanup --provider cf-containers` cannot discover every remote
  container, but it checks local CF Containers claims and removes entries
  whose runner state is expired, stopped, or missing.

## Limitations

- SSH, VNC, browser desktop, code-server, Actions hydration, and `--download`
  are not supported.
- `--fresh-pr` is not supported for delegated archive sync.
- `--checksum` is not supported because the provider uses archive upload and
  extraction instead of Crabbox rsync.
- Container size and concurrency are controlled by
  `worker/wrangler.cloudflare-sandbox.jsonc`. Choose an `instance_type` and
  `max_instances` that match the account's Cloudflare Containers limits.
