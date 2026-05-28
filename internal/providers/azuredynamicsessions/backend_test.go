package azuredynamicsessions

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestRunStopsNewSessionByDefault(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	repo := t.TempDir()
	fake := &recordingAzureDynamicSessionsAPI{}
	restoreAzureDynamicSessionsClient(t, fake)
	backend := testAzureDynamicSessionsBackend()

	result, err := backend.Run(context.Background(), RunRequest{
		Repo:    Repo{Root: repo, Name: "repo"},
		NoSync:  true,
		Command: []string{"printf", "ok"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 || result.Provider != providerName || result.LeaseID == "" {
		t.Fatalf("result = %#v", result)
	}
	if len(fake.deleted) != 1 || fake.deleted[0] != result.LeaseID {
		t.Fatalf("deleted sessions = %#v, want %s", fake.deleted, result.LeaseID)
	}
	if _, ok, err := resolveLeaseClaimForProvider(result.LeaseID, providerName); err != nil || ok {
		t.Fatalf("claim after cleanup ok=%t err=%v", ok, err)
	}
}

func TestRunKeepOnFailureRetainsNewSession(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	repo := t.TempDir()
	fake := &recordingAzureDynamicSessionsAPI{commandExit: 7}
	restoreAzureDynamicSessionsClient(t, fake)
	backend := testAzureDynamicSessionsBackend()

	result, err := backend.Run(context.Background(), RunRequest{
		Repo:          Repo{Root: repo, Name: "repo"},
		NoSync:        true,
		KeepOnFailure: true,
		Command:       []string{"false"},
	})
	var exitErr ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 7 {
		t.Fatalf("err = %v, want exit 7", err)
	}
	if result.LeaseID == "" {
		t.Fatalf("result missing lease: %#v", result)
	}
	if len(fake.deleted) != 0 {
		t.Fatalf("deleted sessions = %#v, want retained session", fake.deleted)
	}
	if claim, ok, err := resolveLeaseClaimForProvider(result.LeaseID, providerName); err != nil || !ok || claim.RepoRoot != repo {
		t.Fatalf("retained claim ok=%t claim=%#v err=%v", ok, claim, err)
	}
}

func TestRunReusesClaimWithoutStoppingSession(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	repo := t.TempDir()
	if err := claimLeaseForRepoProvider("azds-kept", "kept-session", providerName, repo, time.Minute, false); err != nil {
		t.Fatal(err)
	}
	fake := &recordingAzureDynamicSessionsAPI{}
	restoreAzureDynamicSessionsClient(t, fake)
	backend := testAzureDynamicSessionsBackend()

	result, err := backend.Run(context.Background(), RunRequest{
		Repo:    Repo{Root: repo, Name: "repo"},
		ID:      "kept-session",
		NoSync:  true,
		Command: []string{"printf", "ok"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.LeaseID != "azds-kept" || result.Slug != "kept-session" {
		t.Fatalf("result = %#v", result)
	}
	if len(fake.deleted) != 0 {
		t.Fatalf("deleted reused session: %#v", fake.deleted)
	}
}

func TestWarmupRejectsActionsRunner(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	fake := &recordingAzureDynamicSessionsAPI{}
	restoreAzureDynamicSessionsClient(t, fake)
	backend := testAzureDynamicSessionsBackend()

	err := backend.Warmup(context.Background(), WarmupRequest{
		Repo:          Repo{Root: t.TempDir(), Name: "repo"},
		ActionsRunner: true,
	})
	if err == nil || !strings.Contains(err.Error(), "--actions-runner is not supported") {
		t.Fatalf("err = %v, want actions-runner rejection", err)
	}
	if fake.checkRunnerCalls != 0 {
		t.Fatalf("CheckRunner calls = %d, want 0", fake.checkRunnerCalls)
	}
}

func TestStopRemovesStaleClaimWhenSessionIsGone(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := claimLeaseForRepoProvider("azds-stale", "stale-session", providerName, t.TempDir(), time.Minute, false); err != nil {
		t.Fatal(err)
	}
	fake := &recordingAzureDynamicSessionsAPI{
		deleteErr: &azureDynamicSessionsAPIError{StatusCode: 404, Status: "404 Not Found"},
	}
	restoreAzureDynamicSessionsClient(t, fake)
	backend := testAzureDynamicSessionsBackend()

	if err := backend.Stop(context.Background(), StopRequest{ID: "stale-session"}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := resolveLeaseClaimForProvider("stale-session", providerName); err != nil || ok {
		t.Fatalf("claim after stale stop ok=%t err=%v", ok, err)
	}
}

func TestResolveSessionIDRequiresLocalClaim(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	backend := &azureDynamicSessionsBackend{}
	client := &recordingAzureDynamicSessionsAPI{}

	_, _, err := backend.resolveSessionID(context.Background(), client, "azds-external", t.TempDir(), false)
	if err == nil || !strings.Contains(err.Error(), "not claimed by Crabbox") {
		t.Fatalf("resolve unclaimed session err=%v, want claim boundary error", err)
	}
	if client.getSessionCalls != 0 {
		t.Fatalf("GetSession calls = %d, want 0", client.getSessionCalls)
	}
}

func TestResolveSessionIDUsesClaimedSlug(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	repoA := t.TempDir()
	repoB := t.TempDir()
	if err := claimLeaseForRepoProvider("azds-claimed", "claimed-session", providerName, repoA, time.Minute, false); err != nil {
		t.Fatal(err)
	}
	backend := &azureDynamicSessionsBackend{}
	client := &recordingAzureDynamicSessionsAPI{}

	if _, _, err := backend.resolveSessionID(context.Background(), client, "claimed-session", repoB, false); err == nil || !strings.Contains(err.Error(), "use --reclaim") {
		t.Fatalf("resolve without reclaim err=%v, want reclaim guard", err)
	}
	leaseID, slug, err := backend.resolveSessionID(context.Background(), client, "claimed-session", repoB, true)
	if err != nil {
		t.Fatal(err)
	}
	if leaseID != "azds-claimed" || slug != "claimed-session" {
		t.Fatalf("resolved lease=%q slug=%q", leaseID, slug)
	}
	if client.getSessionCalls != 0 {
		t.Fatalf("GetSession calls = %d, want 0", client.getSessionCalls)
	}
}

type recordingAzureDynamicSessionsAPI struct {
	checkRunnerCalls int
	getSessionCalls  int
	deleted          []string
	execs            []azureDynamicSessionsExecRequest
	commandExit      int
	deleteErr        error
}

func (r *recordingAzureDynamicSessionsAPI) CheckRunner(context.Context, string) error {
	r.checkRunnerCalls++
	return nil
}

func (r *recordingAzureDynamicSessionsAPI) UploadFile(context.Context, string, string, string) error {
	return nil
}

func (r *recordingAzureDynamicSessionsAPI) ExecStream(_ context.Context, _ string, req azureDynamicSessionsExecRequest, _ io.Writer, _ io.Writer) (int, error) {
	r.execs = append(r.execs, req)
	if r.commandExit != 0 && !strings.HasPrefix(req.Command, "mkdir -p ") {
		return r.commandExit, nil
	}
	return 0, nil
}

func (r *recordingAzureDynamicSessionsAPI) GetSession(context.Context, string) (azureDynamicSessionsSession, error) {
	r.getSessionCalls++
	return azureDynamicSessionsSession{}, nil
}

func (r *recordingAzureDynamicSessionsAPI) ListSessions(context.Context) ([]azureDynamicSessionsSession, error) {
	return nil, nil
}

func (r *recordingAzureDynamicSessionsAPI) DeleteSession(_ context.Context, identifier string) error {
	r.deleted = append(r.deleted, identifier)
	if r.deleteErr != nil {
		return r.deleteErr
	}
	return nil
}

func restoreAzureDynamicSessionsClient(t *testing.T, api azureDynamicSessionsAPI) {
	t.Helper()
	previous := newAzureDynamicSessionsClient
	newAzureDynamicSessionsClient = func(context.Context, Config, Runtime) (azureDynamicSessionsAPI, error) {
		return api, nil
	}
	t.Cleanup(func() {
		newAzureDynamicSessionsClient = previous
	})
}

func testAzureDynamicSessionsBackend() *azureDynamicSessionsBackend {
	return &azureDynamicSessionsBackend{
		cfg: Config{},
		rt: Runtime{
			Stdout: &bytes.Buffer{},
			Stderr: &bytes.Buffer{},
		},
	}
}
