package api

import "github.com/yuWorm/fba-go/core/plugin"

func Routes(h Handler) []plugin.Route {
	return []plugin.Route{
		plugin.POST("/sys/upload/files", "Upload file", h.UploadFile, plugin.Auth(), plugin.Perm("sys:upload:file:add"), plugin.Tags("uploadfile")),
		plugin.POST("/sys/upload/files/presign", "Create presigned upload", h.CreatePresignedUpload, plugin.Auth(), plugin.Perm("sys:upload:file:add"), plugin.Tags("uploadfile")),
		plugin.POST("/sys/upload/files/:pk/complete", "Complete presigned upload", h.CompletePresignedUpload, plugin.Auth(), plugin.Perm("sys:upload:file:add"), plugin.Tags("uploadfile")),
		plugin.GET("/sys/upload/files/:pk/download", "Download upload file", h.DownloadFile, plugin.Auth(), plugin.Tags("uploadfile")),
		plugin.POST("/sys/upload/files/:pk/access-token", "Create upload file access token", h.CreateFileAccessToken, plugin.Auth(), plugin.Tags("uploadfile")),
		plugin.GET("/sys/upload/files/:pk", "Get upload file", h.GetFile, plugin.Auth(), plugin.Tags("uploadfile")),
		plugin.GET("/sys/upload/files", "List upload files", h.ListFiles, plugin.Auth(), plugin.Tags("uploadfile")),
		plugin.DELETE("/sys/upload/files", "Delete upload files", h.DeleteFiles, plugin.Auth(), plugin.Perm("sys:upload:file:del"), plugin.Tags("uploadfile")),
		plugin.POST("/sys/upload/refs/bind", "Bind upload refs", h.BindRefs, plugin.Auth(), plugin.Perm("sys:upload:ref:bind"), plugin.Tags("uploadfile")),
		plugin.GET("/sys/upload/refs", "List upload refs", h.ListRefs, plugin.Auth(), plugin.Tags("uploadfile")),
		plugin.GET("/sys/upload/scenes", "List upload scenes", h.ListScenes, plugin.Auth(), plugin.Tags("uploadfile")),
		plugin.POST("/sys/upload/scenes", "Create upload scene", h.CreateScene, plugin.Auth(), plugin.Perm("sys:upload:scene:add"), plugin.Tags("uploadfile")),
		plugin.PUT("/sys/upload/scenes/:code", "Update upload scene", h.UpdateScene, plugin.Auth(), plugin.Perm("sys:upload:scene:edit"), plugin.Tags("uploadfile")),
		plugin.DELETE("/sys/upload/scenes/:code", "Delete upload scene", h.DeleteScene, plugin.Auth(), plugin.Perm("sys:upload:scene:del"), plugin.Tags("uploadfile")),
		plugin.GET("/sys/upload/storages", "List upload storages", h.ListStorages, plugin.Auth(), plugin.Tags("uploadfile")),
		plugin.POST("/sys/upload/storages", "Create upload storage", h.CreateStorage, plugin.Auth(), plugin.Perm("sys:upload:storage:add"), plugin.Tags("uploadfile")),
		plugin.PUT("/sys/upload/storages/:code", "Update upload storage", h.UpdateStorage, plugin.Auth(), plugin.Perm("sys:upload:storage:edit"), plugin.Tags("uploadfile")),
		plugin.DELETE("/sys/upload/storages/:code", "Delete upload storage", h.DeleteStorage, plugin.Auth(), plugin.Perm("sys:upload:storage:del"), plugin.Tags("uploadfile")),
		plugin.POST("/sys/upload/shares", "Create upload share", h.CreateShare, plugin.Auth(), plugin.Perm("sys:upload:share:add"), plugin.Tags("uploadfile")),
		plugin.GET("/sys/upload/shares", "List upload shares", h.ListShares, plugin.Auth(), plugin.Tags("uploadfile")),
		plugin.DELETE("/sys/upload/shares/:pk", "Disable upload share", h.DisableShare, plugin.Auth(), plugin.Perm("sys:upload:share:del"), plugin.Tags("uploadfile")),
		plugin.GET("/public/upload/files/:uuid", "Open public upload file", h.OpenPublicFile, plugin.Tags("uploadfile")),
		plugin.GET("/public/upload/shares/:token", "Upload share metadata", h.ShareMetadata, plugin.Tags("uploadfile")),
		plugin.POST("/public/upload/shares/:token/verify", "Verify upload share password", h.VerifySharePassword, plugin.Tags("uploadfile")),
		plugin.GET("/public/upload/shares/:token/download", "Download upload share", h.DownloadShare, plugin.Tags("uploadfile")),
	}
}
