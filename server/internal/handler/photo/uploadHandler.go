package photo

import (
	xhttp "github.com/zeromicro/x/http"
	"net/http"
	"photo-kits-server/server/internal/logic/photo"
	"photo-kits-server/server/internal/svc"
)

func UploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := photo.NewUploadLogic(r.Context(), svcCtx, r, w)
		resp, err := l.Upload()
		if err != nil {
			xhttp.JsonBaseResponseCtx(r.Context(), w, err)
		} else {
			xhttp.JsonBaseResponseCtx(r.Context(), w, resp)
		}
	}
}
