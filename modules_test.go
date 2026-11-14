package gorepo

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/MarkRosemaker/ghrepo"
)

// depFiles is a module that vendors a local one, example.com/dep, of whose
// packages it uses only top.
var depFiles = map[string]string{
	"go.mod": goMod + "\nrequire example.com/dep v0.0.0\n\nreplace example.com/dep => ./dep\n",
	"a.go":   "package a\n\nimport \"example.com/dep/top\"\n\nconst Z = top.X\n",

	"dep/go.mod":     "module example.com/dep\n\ngo 1.26\n",
	"dep/top/top.go": "package top\n\nconst X = 1\n",
	"dep/sub/sub.go": "package sub\n\nconst Y = 2\n",
}

// A test importing a package of a vendored module that vendor/ lacks fails
// until the modules are settled, and passes after.
func TestSettleModules(t *testing.T) {
	repo := newTestRepo(t)

	writeFiles(t, repo, depFiles)

	if err := repo.GoModVendor(t.Context()); err != nil {
		t.Fatal(err)
	}

	writeFiles(t, repo, map[string]string{
		"a_test.go": "package a\n\nimport (\n\t\"testing\"\n\n\t\"example.com/dep/sub\"\n)\n\n" +
			"func TestA(t *testing.T) { _ = sub.Y }\n",
	})

	_, err := repo.GoTestCover(t.Context())
	if !ModulesOutOfStep(err) {
		t.Fatalf("not out of step: %v", err)
	}

	if err := repo.SettleModules(t.Context()); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.GoTestCover(t.Context()); err != nil {
		t.Fatalf("still failing after settling: %v", err)
	}

	if _, err := repo.Stat("vendor/example.com/dep/sub/sub.go"); err != nil {
		t.Errorf("sub not vendored: %v", err)
	}
}

// A module that does not vendor is tidied and left without a vendor/.
func TestSettleModulesDoesNotStartVendoring(t *testing.T) {
	repo := newTestRepo(t)

	writeFiles(t, repo, map[string]string{"go.mod": goMod, "a.go": "package a\n"})

	if err := repo.SettleModules(t.Context()); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.Stat("vendor"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("vendor/ made where there was none: %v", err)
	}
}

func TestSettleModulesFails(t *testing.T) {
	repo := newTestRepo(t)

	writeFiles(t, repo, map[string]string{"go.mod": "not a module\n"})

	if err := repo.SettleModules(t.Context()); err == nil {
		t.Error("tidied a broken go.mod")
	}
}

func TestModulesOutOfStep(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("--- FAIL: TestA\n    a_test.go:4: boom"), false},
		{ghrepo.ExecError{Cmd: "go test", Out: "a_test.go:3:20: cannot find module providing package " +
			"example.com/dep/sub: import lookup disabled by -mod=vendor"}, true},
		{errors.New("go: inconsistent vendoring in /src:"), true},
		{errors.New("a.go:3:8: no required module provides package example.com/x; to add it:"), true},
		{errors.New("missing go.sum entry for module providing package example.com/x"), true},
		{errors.New("go: updates to go.mod needed; to update it:"), true},
	} {
		if got := ModulesOutOfStep(tc.err); got != tc.want {
			t.Errorf("ModulesOutOfStep(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
