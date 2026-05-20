# RunPod Provider

Read when:

- choosing `provider: runpod`;
- pointing Crabbox at a RunPod pod;
- changing `internal/providers/runpod`.

[RunPod](https://runpod.io) is a GPU/CPU cloud whose primitives are pods, GPU
types, CPU flavors, templates, and machines. Crabbox uses the RunPod REST API
at `https://rest.runpod.io/v1` with `Authorization: Bearer $RUNPOD_API_KEY`.

## SSH Lease Shape

A RunPod pod can expose a public IP and a NAT-mapped public port for each
exposed private port. Crabbox creates pods with `ports: ["22/tcp"]` and
`supportPublicIp: true`. RunPod reports the mapping as `publicIp` plus
`portMappings["22"]`; older responses may also expose the same data through
`runtime.ports[]`:

```json
{
  "ip": "203.0.113.7",
  "privatePort": 22,
  "publicPort": 41010,
  "isIpPublic": true,
  "type": "tcp"
}
```

Crabbox provisions the pod through `POST /v1/pods`, polls `GET /v1/pods/{id}`
until public TCP port 22 is available, and then hands the caller a normal
`SSHTarget` pointed at `root@<ip>:<publicPort>`. From that point on, Crabbox
uses its existing SSH sync, run, status, ssh, and stop paths — there is no
parallel RunPod-specific exec surface. RunPod's basic SSH proxy is not used
because it does not support the SCP/SFTP behavior rsync needs.

Public-key SSH authentication is the only supported RunPod auth method. Upload
your ED25519 public key under the RunPod settings page once; RunPod injects it
into every pod you launch. Crabbox does not manage the public key.

## Pod Lifecycle

`Acquire` deploys a pod whose name follows the standard Crabbox
`crabbox-<slug>-<leaseSuffix>` pattern, then waits for `runtime.ports` to
expose a public TCP mapping for port 22. `Resolve` looks up an existing pod by
lease id, pod id, or pod name. `List` enumerates the caller's pods via
`GET /v1/pods` and filters to the `crabbox-` prefix unless `--all` is set.
`ReleaseLease` calls `DELETE /v1/pods/{id}`. `Doctor` runs read-only pod list
checks without ever creating a pod — it is safe to run on every CI.

## Defaults And Cost Discipline

The defaults pick secure-cloud GPU pods on the RunPod PyTorch image with a 20
GB container disk because that path exposes the public TCP SSH mapping Crabbox
needs for rsync. `instanceId` may be a comma-separated GPU priority list; the
default starts with `NVIDIA L4,NVIDIA RTX 4000 Ada Generation,NVIDIA RTX A4000`
and continues through common 24 GB fallback GPUs. Override any of these via
flags, env vars, or YAML if you need a different GPU type or a CPU flavor;
verify that the selected shape exposes public TCP SSH before relying on it.
Pods are terminated immediately on `ReleaseLease`; if `--keep` is set, the pod
stays up and accrues cost until the caller terminates it. Run
`crabbox list --provider runpod` to spot leaked pods.

## Commands

```sh
crabbox warmup --provider runpod
crabbox run --provider runpod -- pnpm test
crabbox ssh --provider runpod
crabbox status --provider runpod
crabbox stop --provider runpod $LEASE_ID
crabbox list --provider runpod
```

## Auth

```sh
export RUNPOD_API_KEY=...   # required, from https://www.runpod.io/console/user/settings
```

`CRABBOX_RUNPOD_API_KEY` is also accepted and wins over `RUNPOD_API_KEY`,
matching the precedence other direct providers use. The key is read from the
environment only; the provider does not register a CLI flag for it. Do not
pass the key on the command line.

The canonical RunPod auth check is:

```sh
curl https://rest.runpod.io/v1/pods \
  -H "Authorization: Bearer $RUNPOD_API_KEY"
```

Crabbox sends the same `Authorization: Bearer $RUNPOD_API_KEY` header to REST
pod endpoints.

## Config

```yaml
provider: runpod
target: linux
runpod:
  apiUrl: https://rest.runpod.io/v1
  cloudType: SECURE       # SECURE | COMMUNITY
  instanceId: NVIDIA L4,NVIDIA RTX 4000 Ada Generation,NVIDIA RTX A4000,NVIDIA GeForce RTX 3090,NVIDIA GeForce RTX 4090,NVIDIA RTX A5000,NVIDIA RTX A4500
  image: runpod/pytorch:2.8.0-py3.11-cuda12.8.1-cudnn-devel-ubuntu22.04
  templateId: ""          # optional
  diskGB: 20
  user: root
  workRoot: /tmp/crabbox
```

Provider flags:

```text
--runpod-url
--runpod-cloud-type
--runpod-instance-id
--runpod-image
--runpod-template-id
--runpod-disk-gb
--runpod-user
--runpod-work-root
```

Environment overrides:

```text
CRABBOX_RUNPOD_API_KEY      (or RUNPOD_API_KEY)
CRABBOX_RUNPOD_API_URL      (or RUNPOD_API_URL)
CRABBOX_RUNPOD_CLOUD_TYPE   (or RUNPOD_CLOUD_TYPE)
CRABBOX_RUNPOD_INSTANCE_ID  (or RUNPOD_INSTANCE_ID)
CRABBOX_RUNPOD_IMAGE        (or RUNPOD_IMAGE)
CRABBOX_RUNPOD_TEMPLATE_ID  (or RUNPOD_TEMPLATE_ID)
CRABBOX_RUNPOD_DISK_GB
CRABBOX_RUNPOD_USER
CRABBOX_RUNPOD_WORK_ROOT
```

## Capabilities

- SSH: yes (public IP + NAT-mapped port 22).
- Crabbox sync: yes (standard rsync-over-SSH).
- Provider sync: no.
- Desktop/browser/code: no.
- Actions hydration: yes (Linux SSH lease).
- Coordinator: no — RunPod runs in direct-only mode.

## Gotchas

- A funded RunPod account is required. `crabbox doctor --provider runpod`
  succeeds on a zero-balance account because it only reads the pod list — the
  balance shortfall only surfaces when an `Acquire` runs.
- You must upload your ED25519 public key to RunPod once before any pod
  bootstrap will accept your SSH session.
- The pod's public port for SSH is allocated at runtime and changes between
  pods; never hard-code `--ssh-port`.
- RunPod's basic SSH proxy is not a Crabbox transport because rsync needs
  SCP/SFTP support.
- `--class` and `--type` are rejected for this provider; use
  `--runpod-instance-id` and `--runpod-image` instead.
- `--tailscale` is rejected; RunPod pods expose only public SSH.

Related docs:

- [Provider backends](../provider-backends.md)
