# doctor

`crabbox doctor` checks local prerequisites and broker/provider access.

```sh
crabbox doctor
crabbox doctor --provider aws
crabbox doctor --provider static-ssh
```

It checks local tools, user config permissions, per-lease key generation support,
coordinator health when configured, and direct-provider API access otherwise. If
`CRABBOX_SSH_KEY` is explicitly set, it also validates that private key and
matching `.pub` file.

With `--provider static-ssh`, it verifies that `static.host` is configured and
probes SSH reachability on the configured host, user, and port.

Flags:

```text
--provider hetzner|aws|static-ssh
```
