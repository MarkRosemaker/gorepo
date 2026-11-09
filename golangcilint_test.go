package gorepo

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/MarkRosemaker/ghrepo"
	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/spf13/afero"
)

func TestGolangCILint_NoPackages(t *testing.T) {
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Skip("golangci-lint not found in PATH")
	}

	repo := newTestRepo(t)

	if err := afero.WriteFile(repo, "go.mod", []byte("module example.com/test\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := repo.GolangCILint(t.Context()); err != nil {
		t.Errorf("unexpected error for repo with no Go files: %v", err)
	}
}

func TestGolangCILint_PropagatesError(t *testing.T) {
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Skip("golangci-lint not found in PATH")
	}

	repo := newTestRepo(t)

	if err := afero.WriteFile(repo, "go.mod", []byte("module example.com/test\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	goSrc := []byte("package main\nimport \"fmt\"\nfunc main() { fmt.Printf(\"%d\", \"not a number\") }\n")
	if err := afero.WriteFile(repo, "main.go", goSrc, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := repo.GolangCILint(t.Context()); err == nil {
		t.Error("expected error for repo with lint violations, got nil")
	}
}

// godot is not among golangci-lint's default linters, so the comment passes only those.
const undottedComment = "package lib\n\n// F returns one\nfunc F() int { return 1 }\n"

func lintRepo(t *testing.T) *Repository {
	t.Helper()

	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Skip("golangci-lint not found in PATH")
	}

	repo := newTestRepo(t)
	writeFiles(t, repo, map[string]string{"go.mod": goMod, "lib.go": undottedComment})

	return repo
}

func TestGolangCILintWithLinters(t *testing.T) {
	repo := lintRepo(t)
	godot := config.Linters{Default: config.GroupNone, Enable: []string{"godot"}}

	// the temporary config file goes to TMPDIR and must not outlive the run
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)

	if err := repo.GolangCILint(t.Context()); err != nil {
		t.Fatalf("default linters: %v", err)
	}

	err := repo.GolangCILintWithLinters(t.Context(), false, godot)
	if execErr := (ghrepo.ExecError{}); !errors.As(err, &execErr) || !strings.Contains(execErr.Out, "godot") {
		t.Fatalf("error = %v, want a godot finding", err)
	}

	if got := readFile(t, repo, "lib.go"); got != undottedComment {
		t.Errorf("lib.go changed without fix:\n%s", got)
	}

	if err := repo.GolangCILintWithLinters(t.Context(), true, godot); err != nil {
		t.Fatalf("with fix: %v", err)
	}

	if got := readFile(t, repo, "lib.go"); !strings.Contains(got, "// F returns one.\n") {
		t.Errorf("lib.go after fix:\n%s", got)
	}

	if entries, err := os.ReadDir(tmp); err != nil || len(entries) != 0 {
		t.Errorf("temporary files left behind: %v, %v", entries, err)
	}
}

// Without explicit linters, the repository's own configuration applies.
func TestGolangCILintFix(t *testing.T) {
	repo := lintRepo(t)
	writeFiles(t, repo, map[string]string{
		".golangci.yaml": "version: \"2\"\nlinters:\n  default: none\n  enable:\n    - godot\n",
	})

	if err := repo.GolangCILint(t.Context()); err == nil {
		t.Fatal("expected the repository's godot to report the comment")
	}

	if err := repo.GolangCILintFix(t.Context()); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, repo, "lib.go"); !strings.Contains(got, "// F returns one.\n") {
		t.Errorf("lib.go after fix:\n%s", got)
	}

	if err := repo.GolangCILint(t.Context()); err != nil {
		t.Errorf("after fix: %v", err)
	}
}
