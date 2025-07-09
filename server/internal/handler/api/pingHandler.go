package api

import (
  "net/http"
  "server/internal/logic/api"

  "server/internal/svc"
  "server/internal/utils"
)

func PingHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return utils.WrapHandler(func(w http.ResponseWriter, r *http.Request) (interface{}, error) {
		l := api.NewPingLogic(r.Context(), svcCtx)
		return l.Ping()
	})
}
