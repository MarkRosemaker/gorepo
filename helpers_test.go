package gorepo

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/MarkRosemaker/ghrepo"
	"github.com/google/go-github/v80/github"
	"github.com/spf13/afero"
)

const goMod = "module example.com/test\n\ngo 1.26\n"

// goEnv holds the caches to keep once HOME points elsewhere.
var goEnv = sync.OnceValues(func() (map[string]string, error) {
	out, err := exec.Command("go", "env", "-json", "GOCACHE", "GOMODCACHE").Output()
	if err != nil {
		return nil, err
	}

	env := map[string]string{}

	return env, json.Unmarshal(out, &env)
})

// isolateEnv keeps the user's git config, HOME and the network away from the commands under test.
func isolateEnv(t *testing.T) {
	t.Helper()

	env, err := goEnv()
	if err != nil {
		t.Fatal(err)
	}

	for k, v := range env {
		t.Setenv(k, v)
	}

	home := t.TempDir()
	config := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)

	// go would otherwise write telemetry counters into home after a test has finished
	telemetry := filepath.Join(config, "go", "telemetry")
	if err := os.MkdirAll(telemetry, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(telemetry, "mode"), []byte("off"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GOPATH", filepath.Join(home, "go"))
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOWORK", "off")
}

// newTestRepo returns a fresh git repository named test/test in a temporary directory.
func newTestRepo(t *testing.T) *Repository {
	t.Helper()

	isolateEnv(t)

	return newRepoIn(t, t.TempDir(), "test", "test")
}

func newRepoIn(t *testing.T, baseDir, owner, name string) *Repository {
	t.Helper()

	ctx := t.Context()

	repo, err := NewService(ctx, "").NewRepository(
		ctx, owner, name,
		ghrepo.WithGithubRepo(&github.Repository{
			Name:  new(name),
			Owner: &github.User{Login: new(owner)},
		}),
		ghrepo.WithBaseDir(baseDir),
		ghrepo.MakeDirAll,
		ghrepo.InitGit,
		ghrepo.CreateRemote,
	)
	if err != nil {
		t.Fatal(err)
	}

	return repo
}

func writeFiles(t *testing.T, fsys afero.Fs, files map[string]string) {
	t.Helper()

	for name, content := range files {
		if err := fsys.MkdirAll(path.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := afero.WriteFile(fsys, name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func readFile(t *testing.T, fsys afero.Fs, name string) string {
	t.Helper()

	data, err := afero.ReadFile(fsys, name)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

// fakeTool puts an executable shell script named name first on PATH.
func fakeTool(t *testing.T, name, script string) {
	t.Helper()

	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not found in PATH")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// libProxy serves example.com/lib at the given versions from a file-based module proxy.
func libProxy(t *testing.T, versions ...string) {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "example.com", "lib", "@v")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	const mod = "module example.com/lib\n\ngo 1.26\n"

	for _, v := range versions {
		files := map[string]string{
			"go.mod":     mod,
			"lib.go":     "package lib\n\nfunc F() int { return 1 }\n",
			"cmd/x/x.go": "package main\n\nfunc main() {}\n",
		}

		buf := &bytes.Buffer{}
		zw := zip.NewWriter(buf)

		for name, content := range files {
			w, err := zw.Create("example.com/lib@" + v + "/" + name)
			if err != nil {
				t.Fatal(err)
			}

			if _, err := w.Write([]byte(content)); err != nil {
				t.Fatal(err)
			}
		}

		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}

		for ext, data := range map[string][]byte{
			".info": fmt.Appendf(nil, `{"Version":%q,"Time":"2026-01-01T00:00:00Z"}`, v),
			".mod":  []byte(mod),
			".zip":  buf.Bytes(),
		} {
			if err := os.WriteFile(filepath.Join(dir, v+ext), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "list"), []byte(strings.Join(versions, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOPROXY", "file://"+filepath.ToSlash(filepath.Dir(filepath.Dir(filepath.Dir(dir)))))
	t.Setenv("GOSUMDB", "off")
	// a cache of its own keeps the fake module out of the user's, and writable for cleanup
	t.Setenv("GOMODCACHE", t.TempDir())
	t.Setenv("GOFLAGS", "-modcacherw")
}
