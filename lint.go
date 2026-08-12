package gorepo

import (
	"context"
	"errors"
	"os"

	"github.com/MarkRosemaker/ghrepo"
	"github.com/golangci/golangci-lint/v2/pkg/config"
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
		data, err := yaml.Marshal(&config.Config{
			Version: "2",
			Linters: *linters,
		})
		if err != nil {
			return err
		}

		tmp, err := os.CreateTemp("", "golangci-*.yml")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name())

		if _, err := tmp.Write(data); err != nil {
			return err
		}
		tmp.Close()

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
