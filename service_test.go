package gorepo

import (
	"testing"

	"github.com/MarkRosemaker/ghrepo"
	"github.com/google/go-github/v80/github"
)

func TestNewService(t *testing.T) {
	svc := NewService(t.Context(), "")
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
}

func TestService_NewRepository(t *testing.T) {
	ctx := t.Context()
	svc := NewService(ctx, "")

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

	if repo == nil {
		t.Fatal("NewRepository returned nil")
	}
}
