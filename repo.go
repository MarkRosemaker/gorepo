package gorepo

import (
	"context"
	"errors"
	"io/fs"

	"github.com/MarkRosemaker/ghrepo"
)

// Repository represents a local go repository.
type Repository struct{ *ghrepo.Repository }

func (r Repository) IsGoRepo() (bool, error) {
	if _, err := r.Stat("go.mod"); err == nil {
		return true, nil
	} else if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else {
		return false, err
	}
}

// GoModInit initializes a go module in the repository.
func (r Repository) GoModInit(ctx context.Context) error {
	_, err := r.ExecCommand(ctx, "go", "mod", "init")
	return err
}

// UpdateDependencies updates all dependencies in the repository.
func (r Repository) UpdateDependencies(ctx context.Context) error {
	if err := r.GoGetAll(ctx); err != nil {
		return err
	}

	if err := r.GoModTidy(ctx); err != nil {
		return err
	}

	return r.GoModVendor(ctx)
}

// GoGetAll updates all dependencies in the repository.
func (r Repository) GoGetAll(ctx context.Context) error {
	_, err := r.ExecCommand(ctx, "go", "get", "-u", "all")
	return err
}

// GoModTidy tidies the go module in the repository.
func (r Repository) GoModTidy(ctx context.Context) error {
	_, err := r.ExecCommand(ctx, "go", "mod", "tidy")
	return err
}

// GoModVendor vendors all dependencies in the repository.
func (r Repository) GoModVendor(ctx context.Context) error {
	_, err := r.ExecCommand(ctx, "go", "mod", "vendor")
	return err
}

func (r Repository) Goimports(ctx context.Context) error {
	_, err := r.ExecCommand(ctx, "goimports", "-w", ".")
	return err
}

func (r Repository) Gofumpt(ctx context.Context) error {
	_, err := r.ExecCommand(ctx, "gofumpt", "-extra", "-w", ".")
	return err
}

func (r Repository) GoFix(ctx context.Context) error {
	if _, err := r.ExecCommand(ctx, "go", "fix", "./..."); err != nil {
		return err
	}

	return nil
}

// GoVet runs go vet on the repository
func (r Repository) GoVet(ctx context.Context) error {
	if _, err := r.ExecCommand(ctx, "go", "vet", "./..."); err != nil {
		const noTestPackagesMsg = "go: warning: \"./...\" matched no packages\nno packages to vet"
		if execErr := (ghrepo.ExecError{}); errors.As(err, &execErr) &&
			execErr.Out == noTestPackagesMsg {
			return nil
		}

		return err
	}

	return nil
}

func (r Repository) GolangCILint(ctx context.Context) error {
	_, err := r.ExecCommand(ctx, "golangci-lint", "run", "./...")
	return err
}
