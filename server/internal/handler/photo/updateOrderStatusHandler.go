package photo

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"server/internal/logic/photo"
	"server/internal/svc"
	"server/internal/types"
)

func UpdateOrderStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OrderUpdateStatusRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := photo.NewUpdateOrderStatusLogic(r.Context(), svcCtx)
		resp, err := l.UpdateOrderStatus(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
