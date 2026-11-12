package verify

import (
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/polyrun"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestHasLocalGoWorkAndBuildEnv_AncestorMembership(t *testing.T) {
	t.Setenv("GOWORK", "")
	t.Setenv("GOTOOLCHAIN", "")

	wsRoot := t.TempDir()
	enrolledRepo := filepath.Join(wsRoot, "app-repo")
	enrolledSub := filepath.Join(enrolledRepo, "substrate")
	brownfieldRepo := filepath.Join(wsRoot, "trials", "scanner")

	mustWriteGoFixture(t, filepath.Join(enrolledSub, "go.mod"), "module example.com/app/substrate\n\ngo 1.26.0\n")
	mustWriteGoFixture(t, filepath.Join(brownfieldRepo, "go.mod"), "module example.com/scanner\n\ngo 1.26.0\n")
	mustWriteGoFixture(t, filepath.Join(wsRoot, "go.work"), strings.Join([]string{
		"go 1.26.0",
		"",
		"// multi-module workspace",
		"use (",
		"\t./app-repo/substrate // enrolled child module",
		"\t\"./other-mod\"",
		")",
	}, "\n"))

	if !astdebt.HasLocalGoWork(enrolledRepo, enrolledSub) {
		t.Fatalf("expected enrolled module %s to resolve active ancestor go.work", enrolledSub)
	}
	enrolledEnv := polyrun.BuildGoToolchainEnv(enrolledRepo, enrolledSub)
	if slices.Contains(enrolledEnv, "GOWORK=off") {
		t.Errorf("expected GOWORK=off to be omitted for enrolled ancestor go.work member, got:\n%s", strings.Join(enrolledEnv, "\n"))
	}

	if astdebt.HasLocalGoWork(brownfieldRepo, brownfieldRepo) {
		t.Fatalf("expected non-enrolled brownfield repo %s to stay isolated from parent go.work", brownfieldRepo)
	}
	brownfieldEnv := polyrun.BuildGoToolchainEnv(brownfieldRepo, brownfieldRepo)
	if !slices.Contains(brownfieldEnv, "GOWORK=off") {
		t.Errorf("expected GOWORK=off for non-enrolled brownfield repo under parent go.work, got:\n%s", strings.Join(brownfieldEnv, "\n"))
	}
}

func TestHasLocalGoWork_DirectNearestShadowAndSyntaxVariants(t *testing.T) {
	t.Run("direct go.work in root", func(t *testing.T) {
		repo := t.TempDir()
		sub := filepath.Join(repo, "substrate")
		mustWriteGoFixture(t, filepath.Join(sub, "go.mod"), "module example.com/direct\n\ngo 1.26.0\n")
		mustWriteGoFixture(t, filepath.Join(repo, "go.work"), "go 1.26.0\nuse ./substrate\n")
		if !astdebt.HasLocalGoWork(repo, sub) {
			t.Fatal("expected direct go.work in root to return true")
		}
	})

	t.Run("nearest ancestor go.work shadows outer go.work", func(t *testing.T) {
		outer := t.TempDir()
		inner := filepath.Join(outer, "sandbox")
		modDir := filepath.Join(inner, "svc")
		mustWriteGoFixture(t, filepath.Join(modDir, "go.mod"), "module example.com/svc\n\ngo 1.26.0\n")
		mustWriteGoFixture(t, filepath.Join(outer, "go.work"), "go 1.26.0\nuse ./sandbox/svc\n")
		mustWriteGoFixture(t, filepath.Join(inner, "go.work"), "go 1.26.0\nuse ./unrelated\n")

		if astdebt.HasLocalGoWork(modDir, modDir) {
			t.Fatal("expected nearest ancestor go.work without modDir to shadow outer go.work")
		}
	})

	t.Run("single-line inline-block and quoted comments", func(t *testing.T) {
		ws := t.TempDir()
		goWork := filepath.Join(ws, "go.work")
		mustWriteGoFixture(t, goWork, strings.Join([]string{
			"go 1.26.0",
			"use ./alpha",
			"use ( `./beta//not-comment` )",
			"use (",
			"  \"./gamma delta\" // trailing comment",
			")",
			"user_note // not a use directive",
		}, "\n"))

		got := astdebt.ParseGoWorkUseDirs(goWork)
		want := []string{"./alpha", "./beta//not-comment", "./gamma delta"}
		if len(got) != len(want) {
			t.Fatalf("parseGoWorkUseDirs count = %d (%v), want %d (%v)", len(got), got, len(want), want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("parseGoWorkUseDirs[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("explicit GOWORK env var preserved", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("GOWORK", "/custom/go.work")
		env := strings.Join(polyrun.BuildGoToolchainEnv(dir, dir), "\n")
		if !strings.Contains(env, "GOWORK=/custom/go.work") {
			t.Errorf("expected explicit GOWORK override, got:\n%s", env)
		}
	})
}
