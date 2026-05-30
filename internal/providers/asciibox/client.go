package asciibox

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type api interface {
	Check(context.Context) error
	CreateBox(context.Context, createRequest) (boxData, error)
	PrepareSSH(context.Context, string) error
	GetBox(context.Context, string) (boxData, error)
	ListBoxes(context.Context) ([]boxData, error)
	DeleteBox(context.Context, string) error
}

type client struct {
	apiKey  string
	apiURL  string
	cliPath string
	home    string
	runner  CommandRunner
}

type createRequest struct {
	TTL time.Duration
}

type boxData struct {
	ID           string `json:"id"`
	Name         string `json:"name,omitempty"`
	State        string `json:"state,omitempty"`
	Status       string `json:"status,omitempty"`
	MachineIP    string `json:"machineIp,omitempty"`
	MachineIPAlt string `json:"machine_ip,omitempty"`
	PublicIP     string `json:"publicIp,omitempty"`
	IP           string `json:"ip,omitempty"`
	SSHUser      string `json:"sshUser,omitempty"`
	SSHUserAlt   string `json:"ssh_user,omitempty"`
	URL          string `json:"url,omitempty"`
	DesktopURL   string `json:"desktopUrl,omitempty"`
	ArchiveAfter any    `json:"archiveAfter,omitempty"`
	ExpiresAt    any    `json:"expiresAt,omitempty"`
	CreatedAt    any    `json:"createdAt,omitempty"`
	UpdatedAt    any    `json:"updatedAt,omitempty"`
}

var newAPI = func(cfg Config, rt Runtime) (api, error) {
	apiKey := strings.TrimSpace(cfg.AsciiBox.APIKey)
	if apiKey == "" {
		return nil, exit(2, "provider=%s requires ASCII_BOX_API_KEY", providerName)
	}
	if rt.Exec == nil {
		return nil, exit(2, "provider=%s requires a local command runner", providerName)
	}
	cliPath := strings.TrimSpace(cfg.AsciiBox.CLIPath)
	if cliPath == "" {
		cliPath = "box"
	}
	apiURL := strings.TrimRight(blank(strings.TrimSpace(cfg.AsciiBox.BaseURL), "https://ascii.dev"), "/")
	return &client{apiKey: apiKey, apiURL: apiURL, cliPath: cliPath, home: asciiBoxCLIHome(), runner: rt.Exec}, nil
}

func (c *client) CreateBox(ctx context.Context, req createRequest) (boxData, error) {
	args := []string{"new"}
	if req.TTL > 0 {
		args = append(args, "--ttl", fmt.Sprintf("%d", int(req.TTL.Round(time.Second).Seconds())))
	}
	result, err := c.run(ctx, args...)
	if err != nil {
		return boxData{}, fmt.Errorf("ascii-box CLI new failed: %s", c.formatError(result, err))
	}
	box, err := decodeNewBox(result.Stdout)
	if err != nil {
		return boxData{}, err
	}
	if strings.TrimSpace(box.ID) == "" {
		return boxData{}, fmt.Errorf("ascii-box CLI new response missing box id")
	}
	return box, nil
}

func (c *client) Check(ctx context.Context) error {
	result, err := c.run(ctx, "limits")
	if err != nil {
		return fmt.Errorf("ascii-box CLI limits failed: %s", c.formatError(result, err))
	}
	return nil
}

func (c *client) PrepareSSH(ctx context.Context, id string) error {
	result, err := c.run(ctx, "ssh", id, "--", "true")
	if err != nil {
		return fmt.Errorf("ascii-box CLI ssh setup failed: %s", c.formatError(result, err))
	}
	return nil
}

func (c *client) GetBox(ctx context.Context, id string) (boxData, error) {
	result, err := c.run(ctx, "info", id)
	if err != nil {
		return boxData{}, fmt.Errorf("ascii-box CLI info failed: %s", c.formatError(result, err))
	}
	return decodeBox([]byte(result.Stdout))
}

func (c *client) ListBoxes(ctx context.Context) ([]boxData, error) {
	result, err := c.run(ctx, "list")
	if err != nil {
		return nil, fmt.Errorf("ascii-box CLI list failed: %s", c.formatError(result, err))
	}
	return decodeBoxes([]byte(result.Stdout))
}

func (c *client) DeleteBox(ctx context.Context, id string) error {
	result, err := c.run(ctx, "delete", id)
	if err != nil {
		return fmt.Errorf("ascii-box CLI delete failed: %s", c.formatError(result, err))
	}
	return nil
}

func (c *client) run(ctx context.Context, args ...string) (LocalCommandResult, error) {
	if err := c.ensureConfig(ctx); err != nil {
		return LocalCommandResult{}, err
	}
	argv := []string{"--no-update", "--json"}
	if c.apiURL != "" {
		argv = append(argv, "--api-url", c.apiURL)
	}
	argv = append(argv, args...)
	return c.runner.Run(ctx, LocalCommandRequest{
		Name: c.cliPath,
		Args: argv,
		Env:  c.env(),
	})
}

func (c *client) ensureConfig(ctx context.Context) error {
	result, err := c.runner.Run(ctx, LocalCommandRequest{
		Name: c.cliPath,
		Args: []string{"--no-update", "--json", "config"},
		Env:  c.env(),
	})
	if err != nil {
		return fmt.Errorf("ascii-box CLI config failed: %s", c.formatError(result, err))
	}
	var cfg struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &cfg); err != nil {
		return fmt.Errorf("decode ascii-box CLI config: %w", err)
	}
	configPath := strings.TrimSpace(cfg.Path)
	if configPath == "" {
		return fmt.Errorf("ascii-box CLI config response missing path")
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(map[string]string{
		"api_url": c.apiURL,
		"token":   c.apiKey,
		"channel": "prod",
	}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		return err
	}
	return nil
}

func (c *client) env() []string {
	return append(setEnv(os.Environ(), "HOME", c.home), "BOX_API_KEY="+c.apiKey)
}

func (c *client) formatError(result LocalCommandResult, err error) string {
	message := strings.TrimSpace(result.Stderr)
	if message == "" {
		message = strings.TrimSpace(result.Stdout)
	}
	if message == "" && err != nil {
		message = err.Error()
	}
	return redactBoxSecrets(blank(message, "unknown error"))
}

var boxSecretRE = regexp.MustCompile(`box_[A-Za-z0-9_-]+`)

func redactBoxSecrets(value string) string {
	return boxSecretRE.ReplaceAllString(value, "box_REDACTED")
}

func asciiBoxCLIHome() string {
	if configured := strings.TrimSpace(os.Getenv("CRABBOX_ASCII_BOX_HOME")); configured != "" {
		return expandUserPath(configured)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".local", "state", "crabbox", "ascii-box")
	}
	return filepath.Join(os.TempDir(), "crabbox-ascii-box")
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	set := false
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			out = append(out, prefix+value)
			set = true
			continue
		}
		out = append(out, entry)
	}
	if !set {
		out = append(out, prefix+value)
	}
	return out
}

func decodeNewBox(output string) (boxData, error) {
	var latest boxData
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var event struct {
			Event string `json:"event"`
			boxData
			Data boxData `json:"data"`
			Box  boxData `json:"box"`
		}
		if err := json.Unmarshal(line, &event); err != nil {
			return boxData{}, fmt.Errorf("decode ascii-box CLI new line: %w", err)
		}
		box := event.boxData
		if box.ID == "" {
			box = event.Data
		}
		if box.ID == "" {
			box = event.Box
		}
		if box.ID != "" {
			latest = mergeBox(latest, box)
		}
		if event.Event == "ready" && latest.ID != "" {
			return latest, nil
		}
		if event.Event == "error" {
			return boxData{}, fmt.Errorf("ascii-box CLI new failed: %s", redactBoxSecrets(string(line)))
		}
	}
	if err := scanner.Err(); err != nil {
		return boxData{}, err
	}
	if latest.ID == "" {
		return boxData{}, fmt.Errorf("decode ascii-box CLI new: no box event")
	}
	return latest, nil
}

func mergeBox(base, update boxData) boxData {
	if update.ID != "" {
		base.ID = update.ID
	}
	if update.Name != "" {
		base.Name = update.Name
	}
	if update.State != "" {
		base.State = update.State
	}
	if update.Status != "" {
		base.Status = update.Status
	}
	if update.IP != "" {
		base.IP = update.IP
	}
	if update.MachineIP != "" {
		base.MachineIP = update.MachineIP
	}
	if update.MachineIPAlt != "" {
		base.MachineIPAlt = update.MachineIPAlt
	}
	if update.PublicIP != "" {
		base.PublicIP = update.PublicIP
	}
	if update.SSHUser != "" {
		base.SSHUser = update.SSHUser
	}
	if update.SSHUserAlt != "" {
		base.SSHUserAlt = update.SSHUserAlt
	}
	if update.URL != "" {
		base.URL = update.URL
	}
	if update.DesktopURL != "" {
		base.DesktopURL = update.DesktopURL
	}
	if update.ArchiveAfter != nil {
		base.ArchiveAfter = update.ArchiveAfter
	}
	if update.CreatedAt != nil {
		base.CreatedAt = update.CreatedAt
	}
	if update.UpdatedAt != nil {
		base.UpdatedAt = update.UpdatedAt
	}
	return base
}

func decodeBox(data []byte) (boxData, error) {
	var wrapped struct {
		Box boxData `json:"box"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && strings.TrimSpace(wrapped.Box.ID) != "" {
		return wrapped.Box, nil
	}
	var box boxData
	if err := json.Unmarshal(data, &box); err != nil {
		return boxData{}, fmt.Errorf("decode ascii-box box: %w", err)
	}
	return box, nil
}

func decodeBoxes(data []byte) ([]boxData, error) {
	var wrapped struct {
		Boxes []boxData `json:"boxes"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.Boxes != nil {
		return wrapped.Boxes, nil
	}
	var boxes []boxData
	if err := json.Unmarshal(data, &boxes); err != nil {
		return nil, fmt.Errorf("decode ascii-box boxes: %w", err)
	}
	return boxes, nil
}
