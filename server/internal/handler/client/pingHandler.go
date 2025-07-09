package client

import (
	"net/http"

	"server/internal/logic/client"
	"server/internal/svc"
	"server/internal/utils"
)

func PingHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return utils.WrapHandler(func(w http.ResponseWriter, r *http.Request) (interface{}, error) {
		l := client.NewPingLogic(r.Context(), svcCtx)
		return l.Ping()
	})
}
