package main

import (
	"embed"

	"github.com/ghuser/ghproject/pkg/config"
	"github.com/ghuser/ghproject/pkg/migrator"
)

//go:embed *.sql
var MigrationsFS embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	if err := migrator.RunMigrations(cfg.DefinitionDatabaseURL, MigrationsFS); err != nil {
		panic(err)
	}
}
