package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/ghuser/ghproject/pkg/app"
	"github.com/ghuser/ghproject/services/item/application/handlers"
	appsvcs "github.com/ghuser/ghproject/services/item/application/services"
)

// ItemRoutes registers item endpoints on the provided chi router.
func ItemRoutes(r chi.Router, a *app.Application) {
	svcs := appsvcs.New(a)
	r.Group(func(r chi.Router) {
		r.Route("/item", func(r chi.Router) {
			r.Post("/", handlers.NewPostItemHandler(svcs).Execute)
		})
	})
}
