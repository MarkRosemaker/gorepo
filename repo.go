package gorepo

import (
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
