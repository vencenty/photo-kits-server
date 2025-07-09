package client

import (
	"net/http"

	"server/internal/logic/client"
	"server/internal/svc"
	"server/internal/utils"
)

func UploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return utils.WrapHandler(func(w http.ResponseWriter, r *http.Request) (interface{}, error) {
		l := client.NewUploadLogic(r.Context(), svcCtx, r, w)
		return l.Upload()
	})
}
