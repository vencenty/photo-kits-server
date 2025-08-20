package api

import (
	"net/http"

	"server/internal/logic/api"
	"server/internal/svc"
	"server/internal/utils"
)

func UploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := api.NewUploadLogic(r.Context(), svcCtx, r, w)
		resp, err := l.Upload()
		utils.HttpResult(r, w, resp, err)
	}
}
