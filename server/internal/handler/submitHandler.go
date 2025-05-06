package handler

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"photo-kits-server/server/internal/logic"
	"photo-kits-server/server/internal/svc"
	"photo-kits-server/server/internal/types"
)

func SubmitHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SubmitRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewSubmitLogic(r.Context(), svcCtx)
		err := l.Submit(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.Ok(w)
		}
	}
}
