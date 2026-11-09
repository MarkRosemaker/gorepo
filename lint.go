package gorepo

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"

	"github.com/MarkRosemaker/ghrepo"
	"github.com/go-viper/mapstructure/v2"
	"github.com/golangci/golangci-lint/v2/pkg/config"
	"gopkg.in/yaml.v3"
)

func (r Repository) GolangCILint(ctx context.Context) error {
	return r.golangCILint(ctx, false, nil)
}

func (r Repository) GolangCILintFix(ctx context.Context) error {
	return r.golangCILint(ctx, true, nil)
}

// GolangCILintWithLinters runs golangci-lint with these linters only; zero-valued settings keep golangci-lint's defaults.
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
		defer os.Remove(tmp.Name()) //nolint:errcheck

		if err := marshalYAML(tmp, &config.Config{
			Version: "2",
			Linters: *linters,
		}); err != nil {
			return err
		}
		defer tmp.Close() //nolint:errcheck

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

	if err := yaml.NewEncoder(w).Encode(dropZero(m)); err != nil {
		return err
	}

	return nil
}

func structToMap(v any) (map[string]any, error) {
	m := map[string]any{}

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "mapstructure",
		Result:  &m,
	})
	if err != nil {
		return nil, err
	}

	if err := decoder.Decode(v); err != nil {
		return nil, err
	}

	return m, nil
}

// dropZero removes zero values, which would otherwise override golangci-lint's own defaults.
func dropZero(v any) any {
	if m, ok := v.(map[string]any); ok {
		for k, e := range m {
			if e = dropZero(e); e == nil {
				delete(m, k)
			} else {
				m[k] = e
			}
		}

		if len(m) == 0 {
			return nil
		}

		return m
	}

	rv := reflect.ValueOf(v)
	if !rv.IsValid() || rv.IsZero() || (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Map) && rv.Len() == 0 {
		return nil
	}

	return v
}
