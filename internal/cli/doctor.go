package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

func (a App) doctor(ctx context.Context, args []string) error {
	fs := newFlagSet("doctor", a.Stderr)
	provider := fs.String("provider", defaultConfig().Provider, "provider: hetzner, aws, or static-ssh")
	id := fs.String("id", "", "remote lease id to inspect")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	ok := true
	for _, tool := range []string{"git", "ssh", "ssh-keygen", "rsync", "curl"} {
		path, err := exec.LookPath(tool)
		if err != nil {
			fmt.Fprintf(a.Stdout, "missing %-8s\n", tool)
			ok = false
			continue
		}
		fmt.Fprintf(a.Stdout, "ok      %-8s %s\n", tool, path)
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if problem := configFilePermissionProblem(writableConfigPath()); problem != "" {
		fmt.Fprintf(a.Stdout, "failed  config   %s: %s\n", writableConfigPath(), problem)
		ok = false
	} else if path := writableConfigPath(); path != "" {
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(a.Stdout, "ok      config   %s permissions=0600\n", path)
		}
	}
	if flagWasSet(fs, "provider") {
		cfg.Provider = *provider
	}
	if *id != "" {
		_, target, _, err := a.resolveLeaseTarget(ctx, cfg, *id)
		if err != nil {
			return err
		}
		out, err := runSSHOutput(ctx, target, "printf 'git='; git --version; printf 'rsync='; rsync --version | head -1; printf 'curl='; curl --version | head -1; printf 'jq='; jq --version")
		if err != nil {
			return exit(7, "remote doctor failed for %s: %v", *id, err)
		}
		fmt.Fprintf(a.Stdout, "ok      remote  %s\n%s\n", *id, out)
	}
	if os.Getenv("CRABBOX_SERVER_TYPE") == "" {
		cfg.ServerType = serverTypeForProviderClass(cfg.Provider, cfg.Class)
	}
	useCoordinator := false
	if coord, coordinatorConfigured, err := newCoordinatorClient(cfg); err != nil {
		fmt.Fprintf(a.Stdout, "failed  coord    %v\n", err)
		ok = false
	} else if coordinatorConfigured {
		useCoordinator = true
		if err := coord.Health(ctx); err != nil {
			fmt.Fprintf(a.Stdout, "failed  coord    %v\n", err)
			ok = false
		} else {
			fmt.Fprintf(a.Stdout, "ok      coord    %s access=%s\n", cfg.Coordinator, accessAuthState(cfg.Access))
			if whoami, err := coord.Whoami(ctx); err != nil {
				fmt.Fprintf(a.Stdout, "failed  broker   %v\n", err)
				ok = false
			} else {
				fmt.Fprintf(a.Stdout, "ok      broker   auth=%s owner=%s org=%s default_type=%s\n", whoami.Auth, whoami.Owner, whoami.Org, cfg.ServerType)
			}
			if cfg.CoordAdminToken != "" {
				adminCfg := cfg
				adminCfg.CoordToken = cfg.CoordAdminToken
				adminCoord, _, err := newCoordinatorClient(adminCfg)
				if err != nil {
					return err
				}
				if machines, err := adminCoord.Pool(ctx, cfg); err != nil {
					fmt.Fprintf(a.Stdout, "failed  admin    %v\n", err)
					ok = false
				} else {
					fmt.Fprintf(a.Stdout, "ok      admin    provider=%s machines=%d\n", cfg.Provider, len(machines))
				}
			}
		}
	}

	if os.Getenv("CRABBOX_SSH_KEY") != "" {
		if _, err := os.Stat(cfg.SSHKey); err != nil {
			fmt.Fprintf(a.Stdout, "missing ssh key %s\n", cfg.SSHKey)
			ok = false
		} else if _, err := publicKeyFor(cfg.SSHKey); err != nil {
			fmt.Fprintf(a.Stdout, "missing ssh public key %s.pub\n", cfg.SSHKey)
			ok = false
		} else {
			fmt.Fprintf(a.Stdout, "ok      ssh-key  %s\n", cfg.SSHKey)
		}
	} else {
		fmt.Fprintf(a.Stdout, "ok      ssh-key  per-lease\n")
	}

	if useCoordinator {
		if !ok {
			return exit(1, "doctor found problems")
		}
		return nil
	}

	switch cfg.Provider {
	case "static-ssh":
		if cfg.StaticSSHHost == "" {
			fmt.Fprintf(a.Stdout, "failed  static-ssh  static.host not configured\n")
			ok = false
		} else {
			target := SSHTarget{
				User:          cfg.SSHUser,
				Host:          cfg.StaticSSHHost,
				Key:           cfg.SSHKey,
				Port:          cfg.SSHPort,
				FallbackPorts: cfg.SSHFallbackPorts,
			}
			if probeSSHReady(ctx, &target, 5*time.Second) {
				fmt.Fprintf(a.Stdout, "ok      static-ssh host=%s user=%s port=%s\n", cfg.StaticSSHHost, cfg.SSHUser, cfg.SSHPort)
			} else {
				fmt.Fprintf(a.Stdout, "failed  static-ssh host=%s user=%s port=%s: SSH not reachable\n", cfg.StaticSSHHost, cfg.SSHUser, cfg.SSHPort)
				ok = false
			}
		}
	case "aws":
		client, err := newAWSClient(ctx, cfg)
		if err != nil {
			fmt.Fprintf(a.Stdout, "failed  aws      %v\n", err)
			ok = false
			break
		}
		servers, err := client.ListCrabboxServers(ctx)
		if err != nil {
			fmt.Fprintf(a.Stdout, "failed  aws      %v\n", err)
			ok = false
		} else {
			fmt.Fprintf(a.Stdout, "ok      aws      crabbox_servers=%d region=%s default_type=%s\n", len(servers), cfg.AWSRegion, cfg.ServerType)
		}
	default:
		client, err := newHetznerClient()
		if err != nil {
			fmt.Fprintf(a.Stdout, "missing hcloud token\n")
			ok = false
		} else {
			servers, err := client.ListCrabboxServers(ctx)
			if err != nil {
				fmt.Fprintf(a.Stdout, "failed  hcloud   %v\n", err)
				ok = false
			} else {
				fmt.Fprintf(a.Stdout, "ok      hcloud   crabbox_servers=%d default_type=%s\n", len(servers), cfg.ServerType)
			}
		}
	}

	if !ok {
		return exit(1, "doctor found problems")
	}
	return nil
}
