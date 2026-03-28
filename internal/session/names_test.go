package session

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/gastownhall/gascity/internal/config"
)

func TestValidateExplicitName(t *testing.T) {
	longName := strings.Repeat("a", explicitSessionNameMaxLen+1)
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "empty allowed", input: "", want: ""},
		{name: "trimmed", input: "  sky  ", want: "sky"},
		{name: "single char", input: "x", want: "x"},
		{name: "bad syntax", input: "sky.chat", wantErr: ErrInvalidSessionName},
		{name: "reserved prefix", input: "s-gc-123", wantErr: ErrInvalidSessionName},
		{name: "too long", input: longName, wantErr: ErrInvalidSessionName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateExplicitName(tt.input)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr.Error()) {
					t.Fatalf("error = %v, want contains %q", err, tt.wantErr.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateAlias(t *testing.T) {
	longAlias := strings.Repeat("a", explicitSessionNameMaxLen+1)
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "empty allowed", input: "", want: ""},
		{name: "trimmed qualified", input: "  myrig/worker  ", want: "myrig/worker"},
		{name: "reserved prefix", input: "s-gc-123", wantErr: ErrInvalidSessionAlias},
		{name: "human reserved", input: "human", wantErr: ErrInvalidSessionAlias},
		{name: "session id syntax reserved", input: "gc-42", wantErr: ErrInvalidSessionAlias},
		{name: "bad syntax", input: "bad:name", wantErr: ErrInvalidSessionAlias},
		{name: "too long", input: longAlias, wantErr: ErrInvalidSessionAlias},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateAlias(tt.input)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr.Error()) {
					t.Fatalf("error = %v, want contains %q", err, tt.wantErr.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUpdatedAliasMetadataPreservesPriorAliases(t *testing.T) {
	got := UpdatedAliasMetadata(map[string]string{
		"alias":         "mayor",
		"alias_history": "witness,sky",
	}, "sky")

	if got["alias"] != "sky" {
		t.Fatalf("alias = %q, want sky", got["alias"])
	}
	if got["alias_history"] != "mayor,witness" {
		t.Fatalf("alias_history = %q, want mayor,witness", got["alias_history"])
	}
}

func TestEnsureSessionNameAvailable_RejectsOpenIdentifierCollisions(t *testing.T) {
	store := beads.NewMemStore()
	open, err := store.Create(beads.Bead{
		Type:   BeadType,
		Labels: []string{LabelSession},
		Metadata: map[string]string{
			"template": "myrig/worker",
		},
	})
	if err != nil {
		t.Fatalf("Create(open): %v", err)
	}

	if err := ensureSessionNameAvailable(store, "worker"); !errors.Is(err, ErrSessionNameExists) {
		t.Fatalf("ensureSessionNameAvailable(open collision) error = %v, want %v", err, ErrSessionNameExists)
	}

	if err := store.Close(open.ID); err != nil {
		t.Fatalf("Close(open): %v", err)
	}
	if err := ensureSessionNameAvailable(store, "worker"); err != nil {
		t.Fatalf("ensureSessionNameAvailable(closed collision) = %v, want nil", err)
	}
}

func TestEnsureSessionNameAvailable_RejectsLiveAliasCollisions(t *testing.T) {
	store := beads.NewMemStore()
	_, err := store.Create(beads.Bead{
		Type:   BeadType,
		Labels: []string{LabelSession},
		Metadata: map[string]string{
			"alias": "worker",
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := ensureSessionNameAvailable(store, "worker"); !errors.Is(err, ErrSessionNameExists) {
		t.Fatalf("ensureSessionNameAvailable(alias collision) error = %v, want %v", err, ErrSessionNameExists)
	}
}

func TestEnsureSessionNameAvailable_RejectsLiveAliasHistoryCollisions(t *testing.T) {
	store := beads.NewMemStore()
	_, err := store.Create(beads.Bead{
		Type:   BeadType,
		Labels: []string{LabelSession},
		Metadata: map[string]string{
			"alias":         "sky",
			"alias_history": "mayor",
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := ensureSessionNameAvailable(store, "mayor"); !errors.Is(err, ErrSessionNameExists) {
		t.Fatalf("ensureSessionNameAvailable(alias history collision) error = %v, want %v", err, ErrSessionNameExists)
	}
}

func TestEnsureAliasAvailableWithConfig_RejectsLiveAliasHistoryCollision(t *testing.T) {
	store := beads.NewMemStore()
	_, err := store.Create(beads.Bead{
		Type:   BeadType,
		Labels: []string{LabelSession},
		Metadata: map[string]string{
			"alias":         "sky",
			"alias_history": "mayor",
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := EnsureAliasAvailable(store, "mayor", ""); !errors.Is(err, ErrSessionAliasExists) {
		t.Fatalf("EnsureAliasAvailable(history collision) error = %v, want %v", err, ErrSessionAliasExists)
	}
}

func TestEnsureAliasAvailable_AllowsClosedSessionNameReuse(t *testing.T) {
	store := beads.NewMemStore()
	bead, err := store.Create(beads.Bead{
		Type:   BeadType,
		Labels: []string{LabelSession},
		Metadata: map[string]string{
			"session_name": "sky",
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.Close(bead.ID); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := EnsureAliasAvailable(store, "sky", ""); err != nil {
		t.Fatalf("EnsureAliasAvailable(closed session_name collision) = %v, want nil", err)
	}
}

func TestEnsureAliasAvailable_RejectsLiveSessionNameCollision(t *testing.T) {
	store := beads.NewMemStore()
	_, err := store.Create(beads.Bead{
		Type:   BeadType,
		Labels: []string{LabelSession},
		Metadata: map[string]string{
			"session_name": "sky",
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := EnsureAliasAvailable(store, "sky", ""); !errors.Is(err, ErrSessionAliasExists) {
		t.Fatalf("EnsureAliasAvailable(live session_name collision) error = %v, want %v", err, ErrSessionAliasExists)
	}
}

func TestEnsureAliasAvailableWithConfig_RejectsConfiguredSingletonAlias(t *testing.T) {
	store := beads.NewMemStore()
	cfg := &config.City{
		NamedSessions: []config.NamedSession{
			{Template: "mayor"},
			{Template: "polecat", Dir: "myrig"},
		},
	}

	if err := EnsureAliasAvailableWithConfig(store, cfg, "myrig/polecat", ""); !errors.Is(err, ErrSessionAliasExists) {
		t.Fatalf("EnsureAliasAvailableWithConfig(reserved singleton) error = %v, want %v", err, ErrSessionAliasExists)
	}
}

func TestEnsureAliasAvailableWithConfig_AllowsConfiguredSingletonSelf(t *testing.T) {
	store := beads.NewMemStore()
	bead, err := store.Create(beads.Bead{
		Type:   BeadType,
		Labels: []string{LabelSession},
		Metadata: map[string]string{
			aliasHistoryMetadataKey:     "old",
			"alias":                     "myrig/polecat",
			"configured_named_session":  "true",
			"configured_named_identity": "myrig/polecat",
			"configured_named_mode":     "on_demand",
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	cfg := &config.City{
		NamedSessions: []config.NamedSession{
			{Template: "polecat", Dir: "myrig"},
		},
	}

	if err := EnsureAliasAvailableWithConfig(store, cfg, "myrig/polecat", bead.ID); err != nil {
		t.Fatalf("EnsureAliasAvailableWithConfig(self singleton) = %v, want nil", err)
	}
}

func TestEnsureAliasAvailableWithConfig_RejectsForkedSingletonSelf(t *testing.T) {
	store := beads.NewMemStore()
	bead, err := store.Create(beads.Bead{
		Type:   BeadType,
		Labels: []string{LabelSession, "template:myrig/polecat"},
		Metadata: map[string]string{
			"template": "myrig/polecat",
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	cfg := &config.City{
		NamedSessions: []config.NamedSession{
			{Template: "polecat", Dir: "myrig"},
		},
	}

	if err := EnsureAliasAvailableWithConfig(store, cfg, "myrig/polecat", bead.ID); !errors.Is(err, ErrSessionAliasExists) {
		t.Fatalf("EnsureAliasAvailableWithConfig(fork self) error = %v, want %v", err, ErrSessionAliasExists)
	}
}

func TestEnsureAliasAvailableWithConfigForOwner_AllowsConfiguredSingletonCreate(t *testing.T) {
	store := beads.NewMemStore()
	cfg := &config.City{
		NamedSessions: []config.NamedSession{
			{Template: "worker"},
			{Template: "polecat", Dir: "myrig"},
		},
	}

	if err := EnsureAliasAvailableWithConfigForOwner(store, cfg, "worker", "", "worker"); err != nil {
		t.Fatalf("EnsureAliasAvailableWithConfigForOwner(worker create) = %v, want nil", err)
	}
	if err := EnsureAliasAvailableWithConfigForOwner(store, cfg, "myrig/polecat", "", "myrig/polecat"); err != nil {
		t.Fatalf("EnsureAliasAvailableWithConfigForOwner(rig singleton create) = %v, want nil", err)
	}
}

func TestEnsureSessionNameAvailable_ClosedLegacyBeadReclaimBySelfOwner(t *testing.T) {
	store := beads.NewMemStore()

	// Create a legacy bead (no configured_named_identity) with an explicit
	// session_name, then close it. This simulates beads created before the
	// named sessions feature existed.
	legacy, err := store.Create(beads.Bead{
		Type:   BeadType,
		Labels: []string{LabelSession},
		Metadata: map[string]string{
			"session_name": "gascity-mayor",
			"agent_name":   "mayor",
		},
	})
	if err != nil {
		t.Fatalf("Create(legacy): %v", err)
	}
	if err := store.Close(legacy.ID); err != nil {
		t.Fatalf("Close(legacy): %v", err)
	}

	// Without selfOwner, closed beads permanently reserve their session_name.
	if err := ensureSessionNameAvailable(store, "gascity-mayor", ""); !errors.Is(err, ErrSessionNameExists) {
		t.Fatalf("ensureSessionNameAvailable(no owner) = %v, want %v", err, ErrSessionNameExists)
	}

	// With selfOwner set, the legacy closed bead (no configured_named_identity)
	// should release its name so the named session can reclaim it.
	if err := ensureSessionNameAvailable(store, "gascity-mayor", "mayor"); err != nil {
		t.Fatalf("ensureSessionNameAvailable(selfOwner=mayor) = %v, want nil", err)
	}
}

func TestEnsureSessionNameAvailable_ClosedNamedBeadReclaimBySelfOwner(t *testing.T) {
	store := beads.NewMemStore()

	// Create a closed configured named session bead with matching identity.
	named, err := store.Create(beads.Bead{
		Type:   BeadType,
		Labels: []string{LabelSession},
		Metadata: map[string]string{
			"session_name":              "gascity-mayor",
			"configured_named_session":  "true",
			"configured_named_identity": "mayor",
		},
	})
	if err != nil {
		t.Fatalf("Create(named): %v", err)
	}
	if err := store.Close(named.ID); err != nil {
		t.Fatalf("Close(named): %v", err)
	}

	// Same identity reclaims.
	if err := ensureSessionNameAvailable(store, "gascity-mayor", "mayor"); err != nil {
		t.Fatalf("ensureSessionNameAvailable(same identity) = %v, want nil", err)
	}

	// Different identity is blocked.
	if err := ensureSessionNameAvailable(store, "gascity-mayor", "deacon"); !errors.Is(err, ErrSessionNameExists) {
		t.Fatalf("ensureSessionNameAvailable(different identity) = %v, want %v", err, ErrSessionNameExists)
	}
}

func TestEnsureSessionNameAvailable_ClosedAdHocBeadRemainsBlocked(t *testing.T) {
	store := beads.NewMemStore()

	// Create an ad-hoc session (no configured_named_session metadata).
	adhoc, err := store.Create(beads.Bead{
		Type:   BeadType,
		Labels: []string{LabelSession},
		Metadata: map[string]string{
			"session_name": "my-custom-session",
		},
	})
	if err != nil {
		t.Fatalf("Create(adhoc): %v", err)
	}
	if err := store.Close(adhoc.ID); err != nil {
		t.Fatalf("Close(adhoc): %v", err)
	}

	// Without selfOwner, ad-hoc session names remain permanently reserved.
	if err := ensureSessionNameAvailable(store, "my-custom-session", ""); !errors.Is(err, ErrSessionNameExists) {
		t.Fatalf("ensureSessionNameAvailable(ad-hoc, no owner) = %v, want %v", err, ErrSessionNameExists)
	}
}

func TestEnsureSessionNameAvailableWithConfig_UsesResolvedWorkspaceName(t *testing.T) {
	store := beads.NewMemStore()
	cfg := &config.City{
		ResolvedWorkspaceName: "test-city",
		Workspace: config.Workspace{
			SessionTemplate: "{{.City}}--{{.Name}}",
		},
		NamedSessions: []config.NamedSession{
			{Template: "mayor"},
		},
	}

	if err := EnsureSessionNameAvailableWithConfig(store, cfg, "test-city--mayor", ""); !errors.Is(err, ErrSessionNameExists) {
		t.Fatalf("EnsureSessionNameAvailableWithConfig(resolved workspace name) error = %v, want %v", err, ErrSessionNameExists)
	}
	if err := EnsureSessionNameAvailableWithConfigForOwner(store, cfg, "test-city--mayor", "", "mayor"); err != nil {
		t.Fatalf("EnsureSessionNameAvailableWithConfigForOwner(self resolved workspace name) = %v, want nil", err)
	}
}

func TestWithCitySessionNameLock_EmptyCityPathFallsBackWithoutLockFile(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir(tmp): %v", err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()

	called := false
	if err := WithCitySessionNameLock("", "sky", func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("WithCitySessionNameLock: %v", err)
	}
	if !called {
		t.Fatal("lock function did not execute")
	}
	if _, err := os.Stat(filepath.Join(tmp, ".gc")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf(".gc should not be created for empty cityPath, got err=%v", err)
	}
}

func TestWithCitySessionLocks_EmptyCityPathSharesIdentifierNamespace(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	aliasDone := make(chan error, 1)
	go func() {
		aliasDone <- WithCitySessionAliasLock("", "sky", func() error {
			close(started)
			<-release
			return nil
		})
	}()
	<-started

	acquired := make(chan struct{})
	nameDone := make(chan error, 1)
	go func() {
		nameDone <- WithCitySessionNameLock("", "sky", func() error {
			close(acquired)
			return nil
		})
	}()

	select {
	case <-acquired:
		t.Fatal("session name lock acquired before alias lock released")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	if err := <-aliasDone; err != nil {
		t.Fatalf("WithCitySessionAliasLock: %v", err)
	}
	if err := <-nameDone; err != nil {
		t.Fatalf("WithCitySessionNameLock: %v", err)
	}
	<-acquired
}
