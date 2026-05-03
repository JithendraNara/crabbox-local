package cli

import (
	"context"
	"fmt"
)

func (a App) ssh(ctx context.Context, args []string) error {
	fs := newFlagSet("ssh", a.Stderr)
	provider := fs.String("provider", defaultConfig().Provider, "provider: hetzner, aws, or static-ssh")
	id := fs.String("id", "", "lease id or slug")
	reclaim := fs.Bool("reclaim", false, "claim this lease for the current repo")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *id == "" && fs.NArg() > 0 {
		*id = fs.Arg(0)
	}
	if *id == "" {
		return exit(2, "usage: crabbox ssh --id <lease-id-or-slug>")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if flagWasSet(fs, "provider") {
		cfg.Provider = *provider
	}
	server, target, leaseID, err := a.resolveLeaseTarget(ctx, cfg, *id)
	if err != nil {
		return err
	}
	repo, err := findRepo()
	if err != nil {
		return err
	}
	if err := claimLeaseForRepo(leaseID, serverSlug(server), repo.Root, cfg.IdleTimeout, *reclaim); err != nil {
		return err
	}
	a.touchActiveLeaseBestEffort(ctx, cfg, server, leaseID)
	fmt.Fprintf(a.Stdout, "ssh -i %s -p %s %s@%s\n", target.Key, target.Port, target.User, target.Host)
	return nil
}
