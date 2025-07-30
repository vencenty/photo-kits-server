package api

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"server/internal/logic/api"
	"server/internal/svc"
	"server/internal/types"
)

func DownloadOssHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DownloadRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := api.NewDownloadOssLogic(r.Context(), svcCtx, r, w)
		err := l.DownloadOss(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		}
		// 注意：这里不需要返回JSON响应，因为logic中已经直接写入了文件流
	}
}
