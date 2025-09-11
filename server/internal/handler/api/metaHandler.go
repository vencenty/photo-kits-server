package api

import (
	"net/http"
	"server/internal/utils"

	"server/internal/logic/api"
	"server/internal/svc"
)

func MetaHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := api.NewMetaLogic(r.Context(), svcCtx, r, w)
		resp, err := l.Meta()
		utils.HttpResult(r, w, resp, err)
	}
}
