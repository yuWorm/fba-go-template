package app

import (
	"github.com/yuWorm/fba-go-template/admin/internal/app/admin"
	"github.com/yuWorm/fba-go-template/admin/internal/app/config"
	"github.com/yuWorm/fba-go-template/admin/internal/app/dict"
	"github.com/yuWorm/fba-go-template/admin/internal/app/notice"
	"github.com/yuWorm/fba-go/core/plugin"
)

func Register(registry *plugin.Registry) error {
	modules := []plugin.Module{
		admin.FBAPlugin(),
		config.FBAPlugin(),
		dict.FBAPlugin(),
		notice.FBAPlugin(),
	}
	for _, module := range modules {
		if err := registry.Add(module, plugin.ModeAuto); err != nil {
			return err
		}
	}
	return nil
}
