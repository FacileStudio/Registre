package docs

import (
	"github.com/FacileStudio/tronc/apiref"
	"github.com/go-chi/chi/v5"
)

func Mount(router chi.Router) {
	apiref.Mount(router, Reference())
}
