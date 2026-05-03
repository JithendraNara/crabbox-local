# list

`crabbox list` shows current Crabbox machines.

```sh
crabbox list
crabbox list --provider aws
crabbox list --provider blacksmith-testbox
crabbox list --json
```

`crabbox pool list` remains as a compatibility alias.

In `blacksmith-testbox` mode this forwards to `blacksmith testbox list`. Human output preserves the Blacksmith table; `--json` emits Crabbox-parsed rows with id, status, repo, workflow, job, ref, and created time when the upstream table exposes those columns.

Flags:

```text
--provider hetzner|aws|static-ssh|static-ssh|blacksmith-testbox
--json
```
