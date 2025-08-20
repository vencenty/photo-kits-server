package api

import (
	"net/http"

	"server/internal/logic/api"
	"server/internal/svc"
	"server/internal/utils"
)

func UploadOssHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := api.NewUploadOssLogic(r.Context(), svcCtx, r, w)
		resp, err := l.UploadOss()
		utils.HttpResult(r, w, resp, err)
	}
}
