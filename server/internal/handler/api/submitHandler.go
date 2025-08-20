package api

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"server/internal/logic/api"
	"server/internal/svc"
	"server/internal/types"
	"server/internal/utils"
)

func SubmitHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SubmitRequest
		if err := httpx.Parse(r, &req); err != nil {
			utils.HttpResult(r, w, nil, err)
			return
		}

		l := api.NewSubmitLogic(r.Context(), svcCtx)
		resp, err := l.Submit(&req)
		utils.HttpResult(r, w, resp, err)
	}
}
