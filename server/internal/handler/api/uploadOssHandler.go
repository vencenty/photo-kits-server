package api

import (
	"net/http"
	"server/internal/utils"

	"server/internal/logic/api"
	"server/internal/svc"
)

func UploadOssHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return utils.WrapHandler(func(w http.ResponseWriter, r *http.Request) (interface{}, error) {
		l := api.NewUploadOssLogic(r.Context(), svcCtx, r, w)
		return l.UploadOss()
	})
}
