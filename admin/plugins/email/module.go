package email

import (
	emailapi "github.com/yuWorm/fba-go-template/admin/plugins/email/api"
	"github.com/yuWorm/fba-go-template/admin/plugins/email/service"
	"github.com/yuWorm/fba-go/core/plugin"
)

func FBAPlugin() plugin.Module {
	return Module{}
}

type Module struct{}

func (Module) Meta() plugin.Meta {
	return plugin.Meta{
		ID:                "email",
		Name:              "Email Plugin",
		Version:           "0.0.3",
		Description:       "Email captcha plugin",
		Author:            "wu-clan",
		Tags:              []string{"other"},
		DependsOn:         []plugin.Dependency{{ID: "admin", Optional: true}},
		AutoInjectDefault: true,
	}
}

func (Module) Register(ctx plugin.Context) error {
	var redisClient service.RedisClient
	_ = ctx.Container().Resolve(&redisClient)

	sender := service.CaptchaSender(service.NoopCaptchaSender{})
	var injectedSender service.CaptchaSender
	if ctx.Container().Resolve(&injectedSender) && injectedSender != nil {
		sender = injectedSender
	}

	handler := emailapi.NewHandler(service.New(service.Options{
		Redis:  redisClient,
		Sender: sender,
	}))
	return plugin.RegisterRoutes(ctx, emailapi.Routes(handler))
}
