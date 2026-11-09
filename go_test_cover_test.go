package gorepo

import (
	"bufio"
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/MarkRosemaker/ghrepo"
)

// A test covers code in other packages too: app's only test covers all of lib.
func TestGoTestCoverCountsEveryPackage(t *testing.T) {
	repo := newTestRepo(t)

	writeFiles(t, repo, map[string]string{
		"go.mod":          goMod,
		"lib/lib.go":      "package lib\n\nfunc Double(n int) int { return 2 * n }\n",
		"app/app.go":      "package app\n\nimport \"example.com/test/lib\"\n\nfunc Four() int { return lib.Double(2) }\n",
		"app/app_test.go": "package app\n\nimport \"testing\"\n\nfunc TestFour(t *testing.T) {\n\tif Four() != 4 {\n\t\tt.Fatal(Four())\n\t}\n}\n",
	})

	got, err := repo.GoTestCover(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if got != 100 {
		t.Errorf("coverage = %v, want 100: lib is covered by app's test", got)
	}
}

func TestGoTestCover(t *testing.T) {
	half := map[string]string{
		"go.mod":       goMod,
		"calc.go":      "package calc\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\nfunc Sub(a, b int) int {\n\treturn a - b\n}\n",
		"calc_test.go": "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal(Add(1, 2))\n\t}\n}\n",
	}

	t.Run("partial", func(t *testing.T) {
		repo := newTestRepo(t)
		writeFiles(t, repo, half)

		got, err := repo.GoTestCover(t.Context())
		if err != nil {
			t.Fatal(err)
		}

		if got != 50 {
			t.Errorf("coverage = %v, want 50: Sub is untested", got)
		}

		if _, err := repo.Stat("cover.out"); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("cover.out left behind: %v", err)
		}
	})

	t.Run("no packages", func(t *testing.T) {
		repo := newTestRepo(t)
		writeFiles(t, repo, map[string]string{"go.mod": goMod})

		if got, err := repo.GoTestCover(t.Context()); err != nil || got != 0 {
			t.Errorf("GoTestCover() = %v, %v, want 0, nil", got, err)
		}
	})

	t.Run("failing test", func(t *testing.T) {
		repo := newTestRepo(t)
		writeFiles(t, repo, half)
		writeFiles(t, repo, map[string]string{
			"sub_test.go": "package calc\n\nimport \"testing\"\n\nfunc TestSub(t *testing.T) {\n\tif Sub(3, 1) != 3 {\n\t\tt.Fatal(\"wrong on purpose\")\n\t}\n}\n",
		})

		_, err := repo.GoTestCover(t.Context())
		if execErr := (ghrepo.ExecError{}); !errors.As(err, &execErr) || !strings.Contains(execErr.Out, "wrong on purpose") {
			t.Errorf("error = %v, want the failing test's output", err)
		}

		if _, err := repo.Stat("cover.out"); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("cover.out left behind after a failure: %v", err)
		}
	})
}

func TestGetTotalLine(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "finds total line",
			input: "pkg/foo.go:10:\tFunc\t100.0%\ntotal:\t(statements)\t85.3%\n",
			want:  "total:\t(statements)\t85.3%",
		},
		{
			name:    "no total line",
			input:   "pkg/foo.go:10:\tFunc\t100.0%\n",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := bufio.NewScanner(strings.NewReader(tt.input))
			got, err := getTotalLine(sc)
			if (err != nil) != tt.wantErr {
				t.Fatalf("getTotalLine() error = %v, wantErr %v", err, tt.wantErr)
			}

			if got != tt.want {
				t.Errorf("getTotalLine() = %q, want %q", got, tt.want)
			}
		})
	}
}
