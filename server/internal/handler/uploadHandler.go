package handler

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"photo-kits-server/server/internal/logic"
	"photo-kits-server/server/internal/svc"
)

func UploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewUploadLogic(r.Context(), svcCtx, r)
		resp, err := l.Upload()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
