# stop

`crabbox stop` releases a coordinator lease or deletes a direct-provider machine.

```sh
crabbox stop blue-lobster
```

`crabbox release` remains as a compatibility alias.
The argument accepts the stable `cbx_...` ID or an active friendly slug. In `blacksmith-testbox` mode it accepts a `tbx_...` ID or local slug and forwards to `blacksmith testbox stop`.

Flags:

```text
--provider hetzner|aws|static-ssh|blacksmith-testbox
```
