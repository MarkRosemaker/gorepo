package gorepo

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MarkRosemaker/ghrepo"
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

	if err := repo.GoVet(t.Context()); err != nil {
		t.Errorf("unexpected error for repo with no Go files: %v", err)
	}
}

func TestDependencies(t *testing.T) {
	repo := newTestRepo(t)
	writeFiles(t, repo, map[string]string{"go.mod": goMod + `
require (
	example.com/a v1.2.3
	example.com/b v0.1.0 // indirect
)
`})

	deps, err := repo.Dependencies()
	if err != nil {
		t.Fatal(err)
	}

	if len(deps) != 2 {
		t.Fatalf("got %d dependencies, want 2", len(deps))
	}

	for i, want := range []struct {
		path, version string
		indirect      bool
	}{
		{"example.com/a", "v1.2.3", false},
		{"example.com/b", "v0.1.0", true},
	} {
		if got := deps[i]; got.Mod.Path != want.path || got.Mod.Version != want.version || got.Indirect != want.indirect {
			t.Errorf("dependency %d = %s %s indirect=%v, want %s %s indirect=%v",
				i, got.Mod.Path, got.Mod.Version, got.Indirect, want.path, want.version, want.indirect)
		}
	}
}

func TestDependencies_Errors(t *testing.T) {
	repo := newTestRepo(t)

	if _, err := repo.Dependencies(); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("without go.mod: error = %v, want fs.ErrNotExist", err)
	}

	writeFiles(t, repo, map[string]string{"go.mod": "module\n"})

	if _, err := repo.Dependencies(); err == nil || !strings.Contains(err.Error(), "parsing go.mod") {
		t.Errorf("with malformed go.mod: error = %v, want a parsing error", err)
	}
}

// go mod init infers the module path from the directory's place in GOPATH.
func TestGoModInit(t *testing.T) {
	isolateEnv(t)

	gopath := t.TempDir()
	t.Setenv("GOPATH", gopath)

	repo := newRepoIn(t, filepath.Join(gopath, "src"), "example.com", "mymod")
	if err := repo.GoModInit(t.Context()); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, repo, "go.mod"); !strings.HasPrefix(got, "module example.com/mymod\n") {
		t.Errorf("go.mod = %q, want module example.com/mymod", got)
	}

	if ok, err := repo.IsGoRepo(); err != nil || !ok {
		t.Errorf("IsGoRepo() = %v, %v after GoModInit", ok, err)
	}
}

func TestGoModInit_OutsideGOPATH(t *testing.T) {
	repo := newTestRepo(t)

	err := repo.GoModInit(t.Context())
	if execErr := (ghrepo.ExecError{}); !errors.As(err, &execErr) || execErr.Cmd != "go mod init" {
		t.Fatalf("error = %v, want an ExecError for go mod init", err)
	}

	if ok, _ := repo.IsGoRepo(); ok {
		t.Error("go.mod created although go mod init failed")
	}
}

// localDepRepo is a module that imports example.com/lib, a module beside it reached by a replace directive.
func localDepRepo(t *testing.T) *Repository {
	t.Helper()

	repo := newTestRepo(t)
	writeFiles(t, repo, map[string]string{
		"go.mod":     goMod + "\nreplace example.com/lib => ./lib\n",
		"main.go":    "package main\n\nimport \"example.com/lib\"\n\nfunc main() { _ = lib.F() }\n",
		"lib/go.mod": "module example.com/lib\n\ngo 1.26\n",
		"lib/lib.go": "package lib\n\nfunc F() int { return 1 }\n",
	})

	return repo
}

func TestGoModTidy(t *testing.T) {
	repo := localDepRepo(t)

	if err := repo.GoModTidy(t.Context()); err != nil {
		t.Fatal(err)
	}

	deps, err := repo.Dependencies()
	if err != nil {
		t.Fatal(err)
	}

	if len(deps) != 1 || deps[0].Mod.Path != "example.com/lib" {
		t.Fatalf("dependencies after tidy = %v, want example.com/lib", deps)
	}

	// once nothing imports it, tidy drops the requirement again
	writeFiles(t, repo, map[string]string{"main.go": "package main\n\nfunc main() {}\n"})

	if err := repo.GoModTidy(t.Context()); err != nil {
		t.Fatal(err)
	}

	if deps, err := repo.Dependencies(); err != nil || len(deps) != 0 {
		t.Errorf("dependencies after second tidy = %v, %v, want none", deps, err)
	}
}

func TestGoModVendor(t *testing.T) {
	repo := localDepRepo(t)

	if err := repo.GoModTidy(t.Context()); err != nil {
		t.Fatal(err)
	}

	if err := repo.GoModVendor(t.Context()); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, repo, "vendor/example.com/lib/lib.go"); !strings.Contains(got, "func F()") {
		t.Errorf("vendored lib.go = %q", got)
	}

	if got := readFile(t, repo, "vendor/modules.txt"); !strings.Contains(got, "# example.com/lib") {
		t.Errorf("modules.txt = %q, want example.com/lib listed", got)
	}
}

// lib v1.0.0 is required, v1.1.0 is the latest.
const requiresOldLib = goMod + "\nrequire example.com/lib v1.0.0\n"

func TestUpdateDependencies(t *testing.T) {
	repo := newTestRepo(t)
	libProxy(t, "v1.0.0", "v1.1.0")
	writeFiles(t, repo, map[string]string{
		"go.mod":  requiresOldLib,
		"main.go": "package main\n\nimport \"example.com/lib\"\n\nfunc main() { _ = lib.F() }\n",
	})

	if err := repo.UpdateDependencies(t.Context()); err != nil {
		t.Fatal(err)
	}

	if deps, err := repo.Dependencies(); err != nil || len(deps) != 1 || deps[0].Mod.Version != "v1.1.0" {
		t.Errorf("dependencies = %v, %v, want example.com/lib v1.1.0", deps, err)
	}

	if got := readFile(t, repo, "vendor/modules.txt"); !strings.Contains(got, "# example.com/lib v1.1.0") {
		t.Errorf("modules.txt = %q, want example.com/lib v1.1.0 vendored", got)
	}
}

// Tools are upgraded although no package imports their module.
func TestUpdateTools(t *testing.T) {
	repo := newTestRepo(t)
	libProxy(t, "v1.0.0", "v1.1.0")
	writeFiles(t, repo, map[string]string{
		"go.mod":  requiresOldLib + "\ntool example.com/lib/cmd/x\n",
		"main.go": "package main\n\nfunc main() {}\n",
	})

	if err := repo.UpdateTools(t.Context()); err != nil {
		t.Fatal(err)
	}

	if deps, err := repo.Dependencies(); err != nil || len(deps) != 1 || deps[0].Mod.Version != "v1.1.0" {
		t.Errorf("dependencies = %v, %v, want example.com/lib v1.1.0 kept for its tool", deps, err)
	}

	if _, err := repo.Stat("vendor/example.com/lib/cmd/x/x.go"); err != nil {
		t.Errorf("tool not vendored: %v", err)
	}
}

// Each update stops at the first command that fails: nothing is vendored without a go.mod.
func TestUpdate_StopsOnError(t *testing.T) {
	for name, update := range map[string]func(*Repository) error{
		"dependencies": func(r *Repository) error { return r.UpdateDependencies(t.Context()) },
		"tools":        func(r *Repository) error { return r.UpdateTools(t.Context()) },
	} {
		t.Run(name, func(t *testing.T) {
			repo := newTestRepo(t)

			err := update(repo)
			if execErr := (ghrepo.ExecError{}); !errors.As(err, &execErr) || !strings.HasPrefix(execErr.Cmd, "go get -u") {
				t.Fatalf("error = %v, want go get to fail", err)
			}

			if _, err := repo.Stat("vendor"); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("vendor exists after a failed update: %v", err)
			}
		})
	}
}

func TestGoimports(t *testing.T) {
	repo := newTestRepo(t)

	log := filepath.Join(t.TempDir(), "log")
	t.Setenv("GOIMPORTS_LOG", log)
	fakeTool(t, "goimports", `echo "$*" >> "$GOIMPORTS_LOG"`)

	writeFiles(t, repo, map[string]string{
		"main.go":           "package main\n",
		"sub/pkg/pkg.go":    "package pkg\n",
		"sub/notes.txt":     "not go\n",
		"README.md":         "# not go\n",
		"vendor/dep/dep.go": "package dep\n",
		".git/hook.go":      "package hook\n",
	})

	if err := repo.Goimports(t.Context()); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}

	got := strings.Fields(string(data))
	slices.Sort(got)

	if want := []string{"-w", "-w", "main.go", "sub/pkg/pkg.go"}; !slices.Equal(got, want) {
		t.Errorf("goimports called with %q, want %q", got, want)
	}
}

func TestGoimports_PropagatesError(t *testing.T) {
	repo := newTestRepo(t)
	fakeTool(t, "goimports", `case "$2" in *bad.go) echo "cannot format $2"; exit 1;; esac`)
	writeFiles(t, repo, map[string]string{"good.go": "package x\n", "bad.go": "package x\n"})

	err := repo.Goimports(t.Context())
	if execErr := (ghrepo.ExecError{}); !errors.As(err, &execErr) || execErr.Out != "cannot format bad.go" {
		t.Errorf("error = %v, want the failure for bad.go", err)
	}
}

func TestGofumpt(t *testing.T) {
	base := t.TempDir()

	isolateEnv(t)

	repo := newRepoIn(t, base, "test", "test")
	fakeTool(t, "gofumpt", `echo "$(pwd) $*"; [ "$GOFUMPT_FAIL" = "" ]`)

	if err := repo.Gofumpt(t.Context()); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOFUMPT_FAIL", "1")

	err := repo.Gofumpt(t.Context())

	execErr := ghrepo.ExecError{}
	if !errors.As(err, &execErr) {
		t.Fatalf("error = %v, want an ExecError", err)
	}

	// the output shows it ran in the repository's root, formatting it all
	if want := filepath.Join(base, "test", "test") + " -extra -w ."; execErr.Out != want {
		t.Errorf("gofumpt ran as %q, want %q", execErr.Out, want)
	}
}

func TestGoFix(t *testing.T) {
	repo := newTestRepo(t)
	writeFiles(t, repo, map[string]string{
		"go.mod": goMod,
		"fix.go": "package fix\n\nfunc F(v interface{}) {\n\tfor i := 0; i < 3; i++ {\n\t\t_ = i\n\t}\n}\n",
	})

	if err := repo.GoFix(t.Context()); err != nil {
		t.Fatal(err)
	}

	want := "package fix\n\nfunc F(v any) {\n\tfor i := range 3 {\n\t\t_ = i\n\t}\n}\n"
	if got := readFile(t, repo, "fix.go"); got != want {
		t.Errorf("fix.go after go fix =\n%s\nwant\n%s", got, want)
	}

	writeFiles(t, repo, map[string]string{"broken.go": "package fix\n\nfunc {\n"})

	if err := repo.GoFix(t.Context()); err == nil {
		t.Error("expected an error for a file that does not parse")
	}
}

func TestGoVet(t *testing.T) {
	repo := newTestRepo(t)
	writeFiles(t, repo, map[string]string{
		"go.mod":  goMod,
		"main.go": "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"ok\") }\n",
	})

	if err := repo.GoVet(t.Context()); err != nil {
		t.Fatalf("clean package: %v", err)
	}

	writeFiles(t, repo, map[string]string{
		"main.go": "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Printf(\"%d\\n\", \"not a number\") }\n",
	})

	err := repo.GoVet(t.Context())
	if execErr := (ghrepo.ExecError{}); !errors.As(err, &execErr) || !strings.Contains(execErr.Out, "Printf format %d") {
		t.Errorf("error = %v, want the Printf finding", err)
	}
}

func TestGoGenerate(t *testing.T) {
	repo := newTestRepo(t)
	writeFiles(t, repo, map[string]string{
		"go.mod": goMod,
		"doc.go": "package test\n\n//go:generate go run gen.go\n",
		"gen.go": "//go:build ignore\n\npackage main\n\nimport \"os\"\n\nfunc main() {\n" +
			"\tif err := os.WriteFile(\"generated.go\", []byte(\"package test\\n\\nconst Answer = 42\\n\"), 0o644); err != nil {\n" +
			"\t\tpanic(err)\n\t}\n}\n",
	})

	if err := repo.GoGenerate(t.Context()); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, repo, "generated.go"); !strings.Contains(got, "const Answer = 42") {
		t.Errorf("generated.go = %q", got)
	}

	writeFiles(t, repo, map[string]string{"doc.go": "package test\n\n//go:generate go run missing.go\n"})

	if err := repo.GoGenerate(t.Context()); err == nil {
		t.Error("expected an error from a failing generator")
	}
}
