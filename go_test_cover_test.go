package gorepo

import (
	"bufio"
	"path"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

// A test covers code in other packages too: app's only test covers all of lib.
func TestGoTestCoverCountsEveryPackage(t *testing.T) {
	repo := newTestRepo(t)

	for name, content := range map[string]string{
		"go.mod":          "module example.com/test\n\ngo 1.26\n",
		"lib/lib.go":      "package lib\n\nfunc Double(n int) int { return 2 * n }\n",
		"app/app.go":      "package app\n\nimport \"example.com/test/lib\"\n\nfunc Four() int { return lib.Double(2) }\n",
		"app/app_test.go": "package app\n\nimport \"testing\"\n\nfunc TestFour(t *testing.T) {\n\tif Four() != 4 {\n\t\tt.Fatal(Four())\n\t}\n}\n",
	} {
		if err := repo.MkdirAll(path.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := afero.WriteFile(repo, name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := repo.GoTestCover(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if got != 100 {
		t.Errorf("coverage = %v, want 100: lib is covered by app's test", got)
	}
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
