package notice

import (
	noticeapi "github.com/yuWorm/fba-go-template/admin/internal/app/notice/api"
	noticemigration "github.com/yuWorm/fba-go-template/admin/internal/app/notice/migration"
	"github.com/yuWorm/fba-go-template/admin/internal/app/notice/repo"
	"github.com/yuWorm/fba-go-template/admin/internal/app/notice/service"
	"github.com/yuWorm/fba-go/core/db"
	"github.com/yuWorm/fba-go/core/plugin"
)

func FBAPlugin() plugin.Module {
	return Module{}
}

type Module struct{}

func (Module) Meta() plugin.Meta {
	return plugin.Meta{
		ID:                "notice",
		Name:              "Notice Plugin",
		Version:           "0.0.2",
		Description:       "System notice and announcement plugin",
		Author:            "wu-clan",
		Tags:              []string{"other"},
		DependsOn:         []plugin.Dependency{{ID: "admin", Optional: true}},
		AutoInjectDefault: true,
	}
}

func (Module) Register(ctx plugin.Context) error {
	repository := repo.Repository(repo.NewMemoryRepository(repo.SeedData()))
	var provider db.Provider
	if ctx.Container().Resolve(&provider) && provider != nil && provider.Write() != nil {
		repository = repo.NewGORMRepository(provider)
		if err := ctx.Migration(noticemigration.AutoMigrate(provider)); err != nil {
			return err
		}
		if err := ctx.Migration(noticemigration.InitialData(provider)); err != nil {
			return err
		}
	}

	handler := noticeapi.NewHandler(service.New(repository))
	return plugin.RegisterRoutes(ctx, noticeapi.Routes(handler))
}
