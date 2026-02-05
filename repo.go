package gorepo

import (
	"github.com/MarkRosemaker/ghrepo"
)

// Repository represents a local go repository.
type Repository struct{ *ghrepo.Repository }
