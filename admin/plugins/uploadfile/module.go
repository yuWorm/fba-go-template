package uploadfile

import (
	uploadapi "github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/api"
	uploadmigration "github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/migration"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/repo"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/service"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/storage"
	"github.com/yuWorm/fba-go/core/db"
	"github.com/yuWorm/fba-go/core/plugin"
)

func FBAPlugin() plugin.Module {
	return Module{}
}

type Module struct{}

func (Module) Meta() plugin.Meta {
	return plugin.Meta{
		ID:                "uploadfile",
		Name:              "Upload File Plugin",
		Version:           "0.1.0",
		Description:       "Unified file upload, share, and management plugin",
		DependsOn:         []plugin.Dependency{{ID: "admin", Optional: true}},
		AutoInjectDefault: true,
	}
}

func (Module) Register(ctx plugin.Context) error {
	repository := repo.Repository(repo.NewMemoryRepository(repo.SeedData()))
	var provider db.Provider
	if ctx.Container().Resolve(&provider) && provider != nil && provider.Write() != nil {
		repository = repo.NewGORMRepository(provider)
		if err := ctx.Migration(uploadmigration.AutoMigrate(provider)); err != nil {
			return err
		}
		if err := ctx.Migration(uploadmigration.InitialData(provider)); err != nil {
			return err
		}
	}

	registry := storage.NewRegistry()
	svc := service.New(repository, registry, service.Options{
		TokenSecret: []byte(ctx.Config().Auth.JWTSecret),
	})
	handler := uploadapi.NewHandler(svc)
	return plugin.RegisterRoutes(ctx, uploadapi.Routes(handler))
}
