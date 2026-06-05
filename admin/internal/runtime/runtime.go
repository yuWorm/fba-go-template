package runtime

import (
	"context"
	"fmt"
	"io"
	"os"

	fba "github.com/yuWorm/fba-go"
	appmodules "github.com/yuWorm/fba-go-template/admin/internal/app"
	"github.com/yuWorm/fba-go/core/command"
	"github.com/yuWorm/fba-go/core/config"
	"github.com/yuWorm/fba-go/core/db"
	"github.com/yuWorm/fba-go/core/di"
	"github.com/yuWorm/fba-go/core/migration"
	"github.com/yuWorm/fba-go/core/plugin"
)

type Options struct {
	Config         fba.Options
	Application    fba.Application
	PluginContext  *plugin.RuntimeContext
	Register       func(*plugin.Registry) error
	NewApplication func(fba.Options) (fba.Application, error)
	OpenDatabase   func(config.DatabaseOptions) (db.Provider, error)
	Out            io.Writer
	Err            io.Writer
}

type Runtime struct {
	config        fba.Options
	application   fba.Application
	pluginContext *plugin.RuntimeContext
	out           io.Writer
	err           io.Writer
}

func New() (*Runtime, error) {
	opts, err := fba.LoadOptionsFromEnv()
	if err != nil {
		return nil, err
	}
	return NewWithOptions(Options{Config: opts})
}

func NewWithOptions(opts Options) (*Runtime, error) {
	cfg := opts.Config.WithDefaults()
	newApplication := opts.NewApplication
	if newApplication == nil {
		newApplication = fba.NewApplication
	}
	application := opts.Application
	var err error
	if application == nil {
		application, err = newApplication(cfg)
		if err != nil {
			return nil, err
		}
	}

	pluginContext := opts.PluginContext
	if pluginContext == nil {
		if err := provideDatabase(application.Container(), cfg.Database, opts.OpenDatabase); err != nil {
			return nil, err
		}
		registry := plugin.NewRegistry()
		register := opts.Register
		if register == nil {
			register = appmodules.Register
		}
		if err := register(registry); err != nil {
			return nil, err
		}
		pluginContext = plugin.NewContext(plugin.ContextOptions{
			Container: application.Container(),
			Router:    application.HTTP(),
			APIGroup:  application.HTTP().Group(cfg.App.APIBasePath),
			Config:    cfg,
		})
		if err := registry.RegisterAll(pluginContext); err != nil {
			return nil, err
		}
		plugin.MountRoutes(pluginContext.APIGroup(), pluginContext.Routes(), plugin.WithContainer(pluginContext.Container()))
	}

	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	errOut := opts.Err
	if errOut == nil {
		errOut = os.Stderr
	}
	return &Runtime{
		config:        cfg,
		application:   application,
		pluginContext: pluginContext,
		out:           out,
		err:           errOut,
	}, nil
}

func (r *Runtime) Execute(ctx context.Context, args []string) error {
	return command.Execute(ctx, command.ExecuteOptions{
		Use:            "admin",
		Short:          r.config.App.Name,
		Runtime:        r,
		Commands:       r.commands(),
		DefaultCommand: "server",
		Out:            r.out,
		Err:            r.err,
	}, args)
}

func (r *Runtime) Container() *di.Container {
	return r.application.Container()
}

func (r *Runtime) Config() config.Options {
	return r.config
}

func (r *Runtime) Output() io.Writer {
	return r.out
}

func (r *Runtime) ErrorOutput() io.Writer {
	return r.err
}

func (r *Runtime) commands() []command.Command {
	commands := []command.Command{
		{
			Use:   "server",
			Short: "Start HTTP server",
			Run: func(ctx context.Context, _ command.Runtime, _ []string) error {
				if r.config.Database.AutoMigrate {
					if err := r.runMigrations(ctx); err != nil {
						return err
					}
				}
				return r.application.Run(ctx)
			},
		},
		{
			Use:   "migrate up",
			Short: "Run database migrations",
			Run: func(ctx context.Context, _ command.Runtime, _ []string) error {
				return r.runMigrations(ctx)
			},
		},
		{
			Use:   "migrate status",
			Short: "Show database migration status",
			Run: func(ctx context.Context, _ command.Runtime, _ []string) error {
				return r.printMigrationStatus(ctx)
			},
		},
	}
	return append(commands, r.pluginContext.Commands()...)
}

func (r *Runtime) runMigrations(ctx context.Context) error {
	provider, err := r.databaseProvider()
	if err != nil {
		return err
	}
	store := migration.NewGORMStore(provider.Write())
	runner := migration.NewRunner(store, migration.NoopLock{})
	return runner.Run(ctx, r.pluginContext.Migrations())
}

func (r *Runtime) printMigrationStatus(ctx context.Context) error {
	provider, err := r.databaseProvider()
	if err != nil {
		return err
	}
	store := migration.NewGORMStore(provider.Write())
	migrations := r.pluginContext.Migrations()
	if len(migrations) == 0 {
		_, err := fmt.Fprintln(r.out, "no migrations registered")
		return err
	}
	for _, item := range migrations {
		applied, err := store.IsApplied(ctx, item.Scope, item.Version)
		if err != nil {
			return err
		}
		marker := "[ ]"
		if applied {
			marker = "[x]"
		}
		if _, err := fmt.Fprintf(r.out, "%s %s %s %s\n", marker, item.Scope, item.Version, item.Name); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) databaseProvider() (db.Provider, error) {
	var provider db.Provider
	if r.application.Container().Resolve(&provider) && provider != nil && provider.Write() != nil {
		return provider, nil
	}
	return nil, fmt.Errorf("database provider is required")
}

func provideDatabase(container *di.Container, opts config.DatabaseOptions, open func(config.DatabaseOptions) (db.Provider, error)) error {
	if opts.Driver == "" && opts.WriteDSN == "" {
		return nil
	}
	if open == nil {
		open = db.Open
	}
	provider, err := open(opts)
	if err != nil {
		return err
	}
	// Register the DB before modules run, because modules attach DB migrations
	// only when they can resolve a database provider from the container.
	return container.Provide(func() db.Provider {
		return provider
	})
}
