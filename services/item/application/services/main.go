package services

import (
	"github.com/ghuser/ghproject/pkg/app"
	"github.com/ghuser/ghproject/pkg/cache"
	"github.com/ghuser/ghproject/services/item/infrastructure/persistence/postgres"
)

// Services is the application-layer service container for this bounded context.
// It wires domain services with their infrastructure implementations.
type Services struct {
	Item *ItemService
}

// New wires all item application services with infrastructure from the Application container.
func New(a *app.Application) *Services {
	repo := postgres.NewItemRepository(a.Db, a.EventBus)
	itemCache := cache.NewItemCache(a.Redis)
	return &Services{
		Item: NewItemService(repo, itemCache),
	}
}
