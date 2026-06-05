package api

import "github.com/yuWorm/fba-go/core/plugin"

func Routes(h Handler) []plugin.Route {
	// Python injects the dict plugin into admin's /sys router, so these paths
	// must keep the /sys prefix for frontend option lookups to hit the same API.
	return []plugin.Route{
		plugin.GET("/sys/dict-types/all", "Get all dict types", h.GetAllDictTypes, plugin.Auth()),
		plugin.GET("/sys/dict-types/:pk", "Get dict type", h.GetDictType, plugin.Auth()),
		plugin.GET("/sys/dict-types", "List dict types", h.ListDictTypes, plugin.Auth()),
		plugin.POST("/sys/dict-types", "Create dict type", h.CreateDictType, plugin.Auth(), plugin.Perm("dict:type:add")),
		plugin.PUT("/sys/dict-types/:pk", "Update dict type", h.UpdateDictType, plugin.Auth(), plugin.Perm("dict:type:edit")),
		plugin.DELETE("/sys/dict-types", "Delete dict types", h.DeleteDictTypes, plugin.Auth(), plugin.Perm("dict:type:del")),
		plugin.GET("/sys/dict-datas/all", "Get all dict data", h.GetAllDictData, plugin.Auth()),
		plugin.GET("/sys/dict-datas/:pk", "Get dict data", h.GetDictData, plugin.Auth()),
		plugin.GET("/sys/dict-datas/type-codes/:code", "Get dict data by type code", h.GetDictDataByTypeCode, plugin.Auth()),
		plugin.GET("/sys/dict-datas", "List dict data", h.ListDictData, plugin.Auth()),
		plugin.POST("/sys/dict-datas", "Create dict data", h.CreateDictData, plugin.Auth(), plugin.Perm("dict:data:add")),
		plugin.PUT("/sys/dict-datas/:pk", "Update dict data", h.UpdateDictData, plugin.Auth(), plugin.Perm("dict:data:edit")),
		plugin.DELETE("/sys/dict-datas", "Delete dict data", h.DeleteDictData, plugin.Auth(), plugin.Perm("dict:data:del")),
	}
}
