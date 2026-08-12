package gorepo

import (
	"bytes"
	"strings"
	"testing"

	"github.com/golangci/golangci-lint/v2/pkg/config"
)

func TestLintersYAMLMarshal(t *testing.T) {
	w := &bytes.Buffer{}
	if err := marshalYAML(w, &config.Config{
		Version: "2",
		Linters: config.Linters{
			Default: config.GroupNone,
			Enable:  []string{"usetesting"},
			Settings: config.LintersSettings{
				UseTesting: config.UseTestingSettings{
					OSCreateTemp:      true,
					OSMkdirTemp:       true,
					OSSetenv:          true,
					OSTempDir:         true,
					OSChdir:           true,
					ContextBackground: true,
				},
			},
		},
	}); err != nil {
		t.Fatalf("marshal: %v", err)
	}

	got := w.String()

	// expected kebab-case keys from mapstructure tags
	for _, want := range []string{
		"version: \"2\"",
		"default: none",
		"enable:",
		"- usetesting",
		"os-create-temp: true",
		"os-mkdir-temp: true",
		"os-setenv: true",
		"os-temp-dir: true",
		"os-chdir: true",
		"context-background: true",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing key/value %q in:\n%s", want, got)
		}
	}
}
