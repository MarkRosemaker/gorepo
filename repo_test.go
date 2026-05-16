package gorepo

import (
	"context"
	"testing"

	"github.com/spf13/afero"
)

func TestIsGoRepo(t *testing.T) {
	repo := newTestRepo(t)

	ok, err := repo.IsGoRepo()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected false without go.mod")
	}

	if err := afero.WriteFile(repo, "go.mod", []byte("module example.com/test\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ok, err = repo.IsGoRepo()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true with go.mod")
	}
}

func TestGoVet_NoPackages(t *testing.T) {
	repo := newTestRepo(t)
	if err := afero.WriteFile(repo, "go.mod", []byte("module example.com/test\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := repo.GoVet(context.Background()); err != nil {
		t.Errorf("unexpected error for repo with no Go files: %v", err)
	}
}
