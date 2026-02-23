package cmd

import (
	"strings"
	"testing"
)

func TestOutputPrimeContext_DogRoleUsesDogTemplate(t *testing.T) {
	ctx := RoleContext{
		Role:     RoleDog,
		TownRoot: "/test/town",
		WorkDir:  "/test/town/deacon/dogs/alpha/gastown",
		Polecat:  "alpha",
	}

	output := captureStdout(t, func() {
		if err := outputPrimeContext(ctx); err != nil {
			t.Fatalf("outputPrimeContext() error = %v", err)
		}
	})

	if !strings.Contains(output, "Dog Context") {
		t.Fatalf("output missing dog context header:\n%s", output)
	}
	if !strings.Contains(output, "/test/town/deacon/dogs/alpha/") {
		t.Fatalf("output missing dog-specific path:\n%s", output)
	}
	if strings.Contains(output, "Could not determine specific role") {
		t.Fatalf("dog role incorrectly fell back to unknown context:\n%s", output)
	}
}

func TestOutputPrimeContext_UnknownRoleFallsBack(t *testing.T) {
	ctx := RoleContext{
		Role:     Role("totally-unknown"),
		TownRoot: "/test/town",
		WorkDir:  "/test/town",
	}

	output := captureStdout(t, func() {
		if err := outputPrimeContext(ctx); err != nil {
			t.Fatalf("outputPrimeContext() error = %v", err)
		}
	})

	if !strings.Contains(output, "Could not determine specific role") {
		t.Fatalf("expected unknown-role fallback output, got:\n%s", output)
	}
}
