package gorepo

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"testing"

	"github.com/MarkRosemaker/ghrepo"
	"github.com/google/go-github/v80/github"
	"golang.org/x/oauth2"
)

func TestNewService(t *testing.T) {
	svc := NewService(t.Context(), "")
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
}

func TestService_NewRepository(t *testing.T) {
	isolateEnv(t)

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

// fakeGitHub serves canned GitHub API responses and records the paths requested.
type fakeGitHub struct {
	mu    sync.Mutex
	paths []string
}

func (f *fakeGitHub) requested() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.paths)
}

// serve answers on behalf of api.github.com in the returned context, which NewService's client picks up.
func (f *fakeGitHub) serve(t *testing.T, routes map[string]http.HandlerFunc) context.Context {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.paths = append(f.paths, r.URL.RequestURI())
		f.mu.Unlock()

		if h, ok := routes[r.URL.Path]; ok {
			h(w, r)
			return
		}

		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	return context.WithValue(t.Context(), oauth2.HTTPClient, &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			r = r.Clone(r.Context())
			r.URL.Scheme, r.URL.Host = target.Scheme, target.Host

			return srv.Client().Transport.RoundTrip(r)
		}),
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// repoPages serves repositories in two pages, linked the way GitHub links them.
func repoPages(first, second string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			_, _ = fmt.Fprintf(w, `[{"name":%q,"description":"second page"}]`, second)
			return
		}

		w.Header().Set("Link", fmt.Sprintf(`<https://api.github.com%s?page=2&per_page=100>; rel="next"`, r.URL.Path))
		_, _ = fmt.Fprintf(w, `[{"name":%q,"description":"first page"}]`, first)
	}
}

func TestService_PrefetchRepositories(t *testing.T) {
	for _, tc := range []struct {
		name     string
		path     string
		prefetch func(*Service, context.Context, string) error
	}{
		{"user", "/users/acme/repos", (*Service).PrefetchUserRepositories},
		{"org", "/orgs/acme/repos", (*Service).PrefetchOrgRepositories},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateEnv(t)

			gh := &fakeGitHub{}
			ctx := gh.serve(t, map[string]http.HandlerFunc{tc.path: repoPages("one", "two")})
			svc := NewService(ctx, "")

			if err := tc.prefetch(svc, ctx, "acme"); err != nil {
				t.Fatal(err)
			}

			want := []string{tc.path + "?page=1&per_page=100", tc.path + "?page=2&per_page=100"}
			if got := gh.requested(); !slices.Equal(got, want) {
				t.Fatalf("requested %q, want %q", got, want)
			}

			// both pages are cached: opening their repositories asks GitHub nothing more
			base := t.TempDir()
			for name, descr := range map[string]string{"one": "first page", "two": "second page"} {
				repo, err := svc.NewRepository(ctx, "acme", name,
					ghrepo.WithBaseDir(base), ghrepo.MakeDirAll, ghrepo.InitGit, ghrepo.CreateRemote)
				if err != nil {
					t.Fatal(err)
				}

				if got := repo.Description(); got != descr {
					t.Errorf("%s: description = %q, want %q", name, got, descr)
				}
			}

			if got := gh.requested(); len(got) != len(want) {
				t.Errorf("requested %q after prefetching, want no more requests", got[len(want):])
			}
		})
	}
}

func TestService_PrefetchRepositories_Error(t *testing.T) {
	isolateEnv(t)

	ctx := (&fakeGitHub{}).serve(t, nil)
	svc := NewService(ctx, "")

	if err := svc.PrefetchUserRepositories(ctx, "nobody"); err == nil {
		t.Error("PrefetchUserRepositories: expected an error for a 404")
	}

	if err := svc.PrefetchOrgRepositories(ctx, "nobody"); err == nil {
		t.Error("PrefetchOrgRepositories: expected an error for a 404")
	}
}

// A repository neither given nor cached is looked up on GitHub.
func TestService_NewRepository_LooksUpGitHub(t *testing.T) {
	isolateEnv(t)

	gh := &fakeGitHub{}
	ctx := gh.serve(t, map[string]http.HandlerFunc{
		"/repos/acme/known": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, `{"name":"known","description":"from the API"}`)
		},
	})
	svc := NewService(ctx, "")
	base := t.TempDir()

	repo, err := svc.NewRepository(ctx, "acme", "known",
		ghrepo.WithBaseDir(base), ghrepo.MakeDirAll, ghrepo.InitGit, ghrepo.CreateRemote)
	if err != nil {
		t.Fatal(err)
	}

	if got := repo.Description(); got != "from the API" {
		t.Errorf("description = %q, want %q", got, "from the API")
	}

	if _, err := svc.NewRepository(ctx, "acme", "missing",
		ghrepo.WithBaseDir(base), ghrepo.MakeDirAll, ghrepo.InitGit, ghrepo.CreateRemote); err == nil {
		t.Error("expected an error for a repository GitHub does not know")
	}

	if got, want := gh.requested(), []string{"/repos/acme/known", "/repos/acme/missing"}; !slices.Equal(got, want) {
		t.Errorf("requested %q, want %q", got, want)
	}
}

func TestService_NewRepository_MissingDir(t *testing.T) {
	isolateEnv(t)

	repo, err := NewService(t.Context(), "").NewRepository(t.Context(), "test", "test",
		ghrepo.WithBaseDir(t.TempDir()))
	if !errors.Is(err, fs.ErrNotExist) || repo != nil {
		t.Errorf("NewRepository() = %v, %v, want nil and fs.ErrNotExist", repo, err)
	}
}
