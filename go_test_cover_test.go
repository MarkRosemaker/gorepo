package gorepo

import (
	"bufio"
	"strings"
	"testing"
)

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
