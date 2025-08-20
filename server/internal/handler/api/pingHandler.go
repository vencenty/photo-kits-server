package api

import (
	"net/http"

	"server/internal/logic/api"
	"server/internal/svc"
	"server/internal/utils"
)

func PingHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := api.NewPingLogic(r.Context(), svcCtx)
		resp, err := l.Ping()
		utils.HttpResult(r, w, resp, err)
	}
}
