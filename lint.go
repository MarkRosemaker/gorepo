package gorepo

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/MarkRosemaker/ghrepo"
	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v3"
)

func (r Repository) GolangCILint(ctx context.Context) error {
	return r.golangCILint(ctx, false, nil)
}

func (r Repository) GolangCILintFix(ctx context.Context) error {
	return r.golangCILint(ctx, true, nil)
}

func (r Repository) GolangCILintWithLinters(ctx context.Context, fix bool, settings config.Linters) error {
	return r.golangCILint(ctx, fix, &settings)
}

func (r Repository) golangCILint(ctx context.Context, fix bool, linters *config.Linters) error {
	args := []string{"run"}
	if fix {
		args = append(args, "--fix")
	}

	if linters != nil {
		tmp, err := os.CreateTemp("", "golangci-*.yml")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name())

		if err := marshalYAML(tmp, &config.Config{
			Version: "2",
			Linters: *linters,
		}); err != nil {
			return err
		}
		defer tmp.Close()

		args = append(args, "-c", tmp.Name())
	}

	if _, err := r.ExecCommand(ctx, "golangci-lint", args...); err != nil {
		const noPackagesMsg = "level=error msg=\"Running error: context loading failed: no go files to analyze: running `go mod tidy` may solve the problem\""
		if execErr := (ghrepo.ExecError{}); errors.As(err, &execErr) &&
			execErr.Out == noPackagesMsg {
			return nil
		}

		return err
	}

	return nil
}

func marshalYAML(w io.Writer, cfg *config.Config) error {
	m, err := structToMap(cfg)
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}

	_, err = w.Write(data)
	return err
}

func structToMap(v any) (map[string]any, error) {
	result := map[string]any{}

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "mapstructure",
		Result:  &result,
	})
	if err != nil {
		return nil, err
	}

	if err := decoder.Decode(v); err != nil {
		return nil, err
	}

	// mapstructure Decode puts into Result, but for nested structs
	// we need recursive cleanup of zero values / proper nesting.
	// Simpler alternative below if this is flaky.
	return result, nil
}
