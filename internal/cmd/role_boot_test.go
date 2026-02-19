package cmd

import (
	"path/filepath"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

func TestParseRoleStringBoot(t *testing.T) {
	tests := []struct {
		input    string
		wantRole Role
		wantRig  string
		wantName string
	}{
		// Simple "boot" → RoleBoot
		{"boot", RoleBoot, "", ""},
		// Compound "deacon/boot" → RoleBoot
		{"deacon/boot", RoleBoot, "", ""},
		// Non-deacon compound should NOT match RoleBoot
		{"west/boot", Role("west/boot"), "", ""},
		// Extra path segments should NOT match RoleBoot
		{"deacon/boot/extra", Role("deacon/boot/extra"), "", ""},
	}

	for _, tt := range tests {
		role, rig, name := parseRoleString(tt.input)
		if role != tt.wantRole {
			t.Errorf("parseRoleString(%q) role = %v, want %v", tt.input, role, tt.wantRole)
		}
		if rig != tt.wantRig {
			t.Errorf("parseRoleString(%q) rig = %q, want %q", tt.input, rig, tt.wantRig)
		}
		if name != tt.wantName {
			t.Errorf("parseRoleString(%q) name = %q, want %q", tt.input, name, tt.wantName)
		}
	}
}

func TestParseRoleStringDog(t *testing.T) {
	tests := []struct {
		input    string
		wantRole Role
		wantRig  string
		wantName string
	}{
		{"dog", RoleDog, "", ""},
		{"dog/alpha", RoleDog, "", "alpha"},
		{"deacon/dogs/alpha", RoleDog, "", "alpha"},
		{"deacon/dogs/boot", RoleBoot, "", ""},
	}

	for _, tt := range tests {
		role, rig, name := parseRoleString(tt.input)
		if role != tt.wantRole {
			t.Errorf("parseRoleString(%q) role = %v, want %v", tt.input, role, tt.wantRole)
		}
		if rig != tt.wantRig {
			t.Errorf("parseRoleString(%q) rig = %q, want %q", tt.input, rig, tt.wantRig)
		}
		if name != tt.wantName {
			t.Errorf("parseRoleString(%q) name = %q, want %q", tt.input, name, tt.wantName)
		}
	}
}

func TestGetRoleHomeBoot(t *testing.T) {
	townRoot := "/tmp/gt"
	got := getRoleHome(RoleBoot, "", "", townRoot)
	want := filepath.Join(townRoot, "deacon", "dogs", "boot")
	if got != want {
		t.Errorf("getRoleHome(RoleBoot) = %q, want %q", got, want)
	}
}

func TestGetRoleHomeDog(t *testing.T) {
	townRoot := "/tmp/gt"
	got := getRoleHome(RoleDog, "", "alpha", townRoot)
	want := filepath.Join(townRoot, "deacon", "dogs", "alpha")
	if got != want {
		t.Errorf("getRoleHome(RoleDog) = %q, want %q", got, want)
	}
}

func TestIsTownLevelRoleBoot(t *testing.T) {
	tests := []struct {
		agentID string
		want    bool
	}{
		{"deacon/boot", true},
		{"deacon-boot", true},
		{"mayor", true},
		{"mayor/", true},
		{"deacon", true},
		{"deacon/", true},
		{"gastown/witness", false},
		{"west/boot", false},
		{"boot", false}, // bare "boot" is not a valid agentID
	}

	for _, tt := range tests {
		got := isTownLevelRole(tt.agentID)
		if got != tt.want {
			t.Errorf("isTownLevelRole(%q) = %v, want %v", tt.agentID, got, tt.want)
		}
	}
}

func TestDetectRoleDogFromCwd(t *testing.T) {
	townRoot := "/tmp/gt"
	cwd := filepath.Join(townRoot, "deacon", "dogs", "alpha", "gastown")

	info := detectRole(cwd, townRoot)
	if info.Role != RoleDog {
		t.Fatalf("detectRole(%q) role = %v, want %v", cwd, info.Role, RoleDog)
	}
	if info.Polecat != "alpha" {
		t.Fatalf("detectRole(%q) name = %q, want %q", cwd, info.Polecat, "alpha")
	}
}

func TestDetectRoleBootFromCwd(t *testing.T) {
	townRoot := "/tmp/gt"
	cwd := filepath.Join(townRoot, "deacon", "dogs", "boot", "gastown")

	info := detectRole(cwd, townRoot)
	if info.Role != RoleBoot {
		t.Fatalf("detectRole(%q) role = %v, want %v", cwd, info.Role, RoleBoot)
	}
}

func TestActorStringBoot(t *testing.T) {
	info := RoleInfo{Role: RoleBoot}
	got := info.ActorString()
	want := "deacon-boot"
	if got != want {
		t.Errorf("ActorString() for RoleBoot = %q, want %q", got, want)
	}
}

func TestActorStringDog(t *testing.T) {
	info := RoleInfo{Role: RoleDog, Polecat: "alpha"}
	got := info.ActorString()
	want := "dog/alpha"
	if got != want {
		t.Errorf("ActorString() for RoleDog = %q, want %q", got, want)
	}
}

func TestActorStringConsistentWithBDActorBoot(t *testing.T) {
	// ActorString() must match what BD_ACTOR is set to in config/env.go:57.
	// This is a snapshot value — if BD_ACTOR for boot changes in config/env.go,
	// update it here too.
	info := RoleInfo{Role: RoleBoot}
	actorString := info.ActorString()
	bdActor := "deacon-boot" // snapshot from internal/config/env.go:57
	if actorString != bdActor {
		t.Errorf("ActorString() = %q does not match BD_ACTOR = %q", actorString, bdActor)
	}
}

func TestActorStringConsistentWithBDActorDog(t *testing.T) {
	// ActorString() must match what BD_ACTOR is set to in config/env.go for role dog.
	info := RoleInfo{Role: RoleDog, Polecat: "alpha"}
	actorString := info.ActorString()
	bdActor := "dog/alpha"
	if actorString != bdActor {
		t.Errorf("ActorString() = %q does not match BD_ACTOR = %q", actorString, bdActor)
	}
}

func TestGetRoleWithContextDogEnvAndName(t *testing.T) {
	t.Setenv(EnvGTRole, "dog")
	t.Setenv("GT_DOG", "alpha")

	townRoot := "/tmp/gt"
	cwd := filepath.Join(townRoot, "deacon", "dogs", "alpha", "gastown")

	info, err := GetRoleWithContext(cwd, townRoot)
	if err != nil {
		t.Fatalf("GetRoleWithContext() error = %v", err)
	}
	if info.Role != RoleDog {
		t.Fatalf("Role = %v, want %v", info.Role, RoleDog)
	}
	if info.Polecat != "alpha" {
		t.Fatalf("Polecat = %q, want %q", info.Polecat, "alpha")
	}
	if info.EnvIncomplete {
		t.Fatalf("EnvIncomplete = true, want false")
	}
}

func TestGetRoleWithContextDogEnvFilledFromCwd(t *testing.T) {
	t.Setenv(EnvGTRole, "dog")
	t.Setenv("GT_DOG", "")

	townRoot := "/tmp/gt"
	cwd := filepath.Join(townRoot, "deacon", "dogs", "alpha", "gastown")

	info, err := GetRoleWithContext(cwd, townRoot)
	if err != nil {
		t.Fatalf("GetRoleWithContext() error = %v", err)
	}
	if info.Role != RoleDog {
		t.Fatalf("Role = %v, want %v", info.Role, RoleDog)
	}
	if info.Polecat != "alpha" {
		t.Fatalf("Polecat = %q, want %q", info.Polecat, "alpha")
	}
	if !info.EnvIncomplete {
		t.Fatalf("EnvIncomplete = false, want true")
	}
}

func TestBuildAgentBeadIDBoot(t *testing.T) {
	// RoleBoot should produce the town-level dog bead ID "hq-dog-boot"
	// via both the explicit role path and the identity-inference path.
	want := beads.DogBeadIDTown("boot")

	// Explicit role path
	got := buildAgentBeadID("deacon-boot", RoleBoot, "/tmp/gt")
	if got != want {
		t.Errorf("buildAgentBeadID(RoleBoot) = %q, want %q", got, want)
	}

	// Identity inference path (RoleUnknown + "deacon-boot" identity)
	got = buildAgentBeadID("deacon-boot", RoleUnknown, "/tmp/gt")
	if got != want {
		t.Errorf("buildAgentBeadID(RoleUnknown, \"deacon-boot\") = %q, want %q", got, want)
	}
}
