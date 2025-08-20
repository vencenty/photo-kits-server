package api

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"server/internal/logic/api"
	"server/internal/svc"
	"server/internal/types"
	"server/internal/utils"
)

func OrderInfoHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OrderInfoRequest
		if err := httpx.Parse(r, &req); err != nil {
			utils.HttpResult(r, w, nil, err)
			return
		}

		l := api.NewOrderInfoLogic(r.Context(), svcCtx)
		resp, err := l.OrderInfo(&req)
		utils.HttpResult(r, w, resp, err)
	}
}
