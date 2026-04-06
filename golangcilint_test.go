package gorepo

import (
	"context"
	"os/exec"
	"testing"

	"github.com/MarkRosemaker/ghrepo"
	"github.com/google/go-github/v80/github"
	"github.com/spf13/afero"
)

func newTestRepo(t *testing.T) *Repository {
	t.Helper()

	ctx := context.Background()
	svc := ghrepo.NewService(ctx, "")
	repo, err := svc.NewRepository(ctx, "test", "test",
		ghrepo.WithGithubRepo(&github.Repository{
			Name:  new("test"),
			Owner: &github.User{Login: new("test")},
		}),
		ghrepo.WithBaseDir(t.TempDir()),
		ghrepo.MakeDirAll,
		ghrepo.InitGit,
		ghrepo.CreateRemote,
	)
	if err != nil {
		t.Fatal(err)
	}

	return &Repository{Repository: repo}
}

func TestGolangCILint_NoPackages(t *testing.T) {
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Skip("golangci-lint not found in PATH")
	}

	repo := newTestRepo(t)

	if err := repo.GolangCILint(context.Background()); err != nil {
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

	if err := repo.GolangCILint(context.Background()); err == nil {
		t.Error("expected error for repo with lint violations, got nil")
	}
}
