package uploadfile

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	uploadapi "github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/api"
	uploadconfig "github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/config"
	uploadmigration "github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/migration"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/repo"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/service"
	"github.com/yuWorm/fba-go-template/admin/plugins/uploadfile/storage"
	"github.com/yuWorm/fba-go/core/command"
	"github.com/yuWorm/fba-go/core/db"
	"github.com/yuWorm/fba-go/core/plugin"
	coretask "github.com/yuWorm/fba-go/core/task"
)

const (
	cleanupTaskType  = "uploadfile.cleanup"
	cleanupTaskName  = "Cleanup expired upload files"
	cleanupTaskQueue = "default"
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
	configOptions, err := uploadconfig.Load(uploadconfig.LoadOptions{})
	if err != nil {
		return err
	}
	seed, err := uploadconfig.ApplyToSeed(repo.SeedData(), configOptions)
	if err != nil {
		return err
	}

	repository := repo.Repository(repo.NewMemoryRepository(seed))
	var provider db.Provider
	if ctx.Container().Resolve(&provider) && provider != nil && provider.Write() != nil {
		repository = repo.NewGORMRepository(provider)
		if err := ctx.Migration(uploadmigration.AutoMigrate(provider)); err != nil {
			return err
		}
		if err := ctx.Migration(uploadmigration.InitialData(provider, seed)); err != nil {
			return err
		}
	}

	registry := storage.NewRegistry()
	svc := service.New(repository, registry, service.Options{
		TokenSecret: []byte(ctx.Config().Auth.JWTSecret),
	})
	if err := ctx.Task(plugin.TaskDefinition{
		Type:  cleanupTaskType,
		Name:  cleanupTaskName,
		Queue: cleanupTaskQueue,
	}); err != nil {
		return err
	}
	var taskRegistry coretask.DefinitionRegistry
	if ctx.Container().Resolve(&taskRegistry) && taskRegistry != nil {
		if err := taskRegistry.Add(coretask.Definition{
			Type:  cleanupTaskType,
			Name:  cleanupTaskName,
			Queue: cleanupTaskQueue,
			Handler: asynq.HandlerFunc(func(taskCtx context.Context, _ *asynq.Task) error {
				_, err := svc.CleanupExpiredTemps(taskCtx, service.CleanupOptions{})
				return err
			}),
		}); err != nil {
			return err
		}
	}
	if err := ctx.Command(command.Command{
		Use:                "uploadfile cleanup",
		Short:              "Cleanup expired temporary upload files",
		DisableFlagParsing: true,
		Run: func(ctx context.Context, runtime command.Runtime, args []string) error {
			dryRun := false
			for _, arg := range args {
				if arg == "--dry-run" {
					dryRun = true
				}
			}
			result, err := svc.CleanupExpiredTemps(ctx, service.CleanupOptions{DryRun: dryRun})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(runtime.Output(), "expired_refs=%d deleted_files=%d dry_run=%t\n", result.ExpiredRefs, result.DeletedFiles, dryRun)
			return err
		},
	}); err != nil {
		return err
	}
	handler := uploadapi.NewHandler(svc)
	return plugin.RegisterRoutes(ctx, uploadapi.Routes(handler))
}
