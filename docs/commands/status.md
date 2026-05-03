# status

`crabbox status` prints the current state for a lease.

```sh
crabbox status --id blue-lobster
crabbox status --id blue-lobster --wait --wait-timeout 10m
crabbox status --id blue-lobster --json
```

`--id` accepts the canonical `cbx_...` ID or active slug. In `blacksmith-testbox` mode it accepts a `tbx_...` ID or local slug and forwards to `blacksmith testbox status`. Plain status is read-only; `--wait` touches the lease while waiting for Crabbox brokered leases.

Flags:

```text
--id <lease-id-or-slug>
--provider hetzner|aws|static-ssh|blacksmith-testbox
--wait
--wait-timeout <duration>
--json
```
