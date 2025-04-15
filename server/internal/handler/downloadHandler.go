package handler

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"photo-kits-server/server/internal/logic"
	"photo-kits-server/server/internal/svc"
	"photo-kits-server/server/internal/types"
)

func DownloadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DownloadRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewDownloadLogic(r.Context(), svcCtx)
		err := l.Download(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.Ok(w)
		}
	}
}
