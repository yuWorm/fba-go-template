package app_test

import (
	"testing"

	emailplugin "github.com/yuWorm/fba-go-template/admin/plugins/email"
	oauth2plugin "github.com/yuWorm/fba-go-template/admin/plugins/oauth2"
	taskplugin "github.com/yuWorm/fba-go-template/admin/plugins/task"
	uploadfileplugin "github.com/yuWorm/fba-go-template/admin/plugins/uploadfile"
)

func TestReferencePluginsLiveUnderTemplatePlugins(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   string
	}{
		{name: "email", id: emailplugin.FBAPlugin().Meta().ID},
		{name: "oauth2", id: oauth2plugin.FBAPlugin().Meta().ID},
		{name: "task", id: taskplugin.FBAPlugin().Meta().ID},
		{name: "uploadfile", id: uploadfileplugin.FBAPlugin().Meta().ID},
	} {
		if tc.name != tc.id {
			t.Fatalf("plugin %q meta ID = %q", tc.name, tc.id)
		}
	}
}
