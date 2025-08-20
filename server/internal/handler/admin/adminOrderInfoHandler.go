package admin

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"server/internal/logic/admin"
	"server/internal/svc"
	"server/internal/types"
	"server/internal/utils"
)

func AdminOrderInfoHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OrderInfoRequest
		if err := httpx.Parse(r, &req); err != nil {
			utils.HttpResult(r, w, nil, err)
			return
		}

		l := admin.NewAdminOrderInfoLogic(r.Context(), svcCtx)
		resp, err := l.AdminOrderInfo(&req)
		utils.HttpResult(r, w, resp, err)
	}
}
