package dict

import (
	dictapi "github.com/yuWorm/fba-go-template/admin/internal/app/dict/api"
	dictmigration "github.com/yuWorm/fba-go-template/admin/internal/app/dict/migration"
	"github.com/yuWorm/fba-go-template/admin/internal/app/dict/repo"
	"github.com/yuWorm/fba-go-template/admin/internal/app/dict/service"
	"github.com/yuWorm/fba-go/core/db"
	"github.com/yuWorm/fba-go/core/plugin"
	"github.com/yuWorm/fba-go/core/redisx"
)

func FBAPlugin() plugin.Module {
	return Module{}
}

type Module struct{}

func (Module) Meta() plugin.Meta {
	return plugin.Meta{
		ID:                "dict",
		Name:              "Dict Plugin",
		Version:           "0.0.8",
		Description:       "Dictionary data plugin",
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
		if err := ctx.Migration(dictmigration.AutoMigrate(provider)); err != nil {
			return err
		}
		if err := ctx.Migration(dictmigration.InitialData(provider)); err != nil {
			return err
		}
	}

	invalidator := service.CacheInvalidator(service.NoopInvalidator{})
	var redisClient redisx.RedisClient
	if ctx.Container().Resolve(&redisClient) && redisClient != nil {
		keys := redisx.NewKeys(ctx.Config().Redis.KeyPrefix)
		invalidator = service.NewRedisInvalidator(redisClient, keys.CacheInvalidateChannel(), keys.DictCache())
	}

	handler := dictapi.NewHandler(service.New(repository, invalidator))
	return plugin.RegisterRoutes(ctx, dictapi.Routes(handler))
}
