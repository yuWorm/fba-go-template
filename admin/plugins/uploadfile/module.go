package uploadfile

import (
	"context"
	"fmt"

	uploadapi "github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/api"
	uploadmigration "github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/migration"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/repo"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/service"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/storage"
	"github.com/yuWorm/fba-go/core/command"
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
	if err := ctx.Command(command.Command{
		Use:   "uploadfile cleanup",
		Short: "Cleanup expired temporary upload files",
		Run: func(ctx context.Context, runtime command.Runtime, _ []string) error {
			result, err := svc.CleanupExpiredTemps(ctx)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(runtime.Output(), "expired_refs=%d deleted_files=%d\n", result.ExpiredRefs, result.DeletedFiles)
			return err
		},
	}); err != nil {
		return err
	}
	handler := uploadapi.NewHandler(svc)
	return plugin.RegisterRoutes(ctx, uploadapi.Routes(handler))
}
