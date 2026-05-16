package gorepo

import (
	"context"
	"testing"

	"github.com/MarkRosemaker/ghrepo"
	"github.com/google/go-github/v80/github"
)

func TestNewService(t *testing.T) {
	svc := NewService(context.Background(), "")
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
}

func TestService_NewRepository(t *testing.T) {
	ctx := context.Background()
	svc := NewService(ctx, "")
	repo, err := svc.NewRepository(ctx, "test", "test",
		ghrepo.WithGithubRepo(&github.Repository{
			Name:  github.String("test"),
			Owner: &github.User{Login: github.String("test")},
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
