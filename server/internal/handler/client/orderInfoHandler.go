package client

import (
	"net/http"

	"server/internal/logic/client"
	"server/internal/svc"
	"server/internal/types"
	"server/internal/utils"
)

func OrderInfoHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return utils.WrapHandlerWithRequest(func(w http.ResponseWriter, r *http.Request, req interface{}) (interface{}, error) {
		l := client.NewOrderInfoLogic(r.Context(), svcCtx)
		return l.OrderInfo(req.(*types.OrderInfoRequest))
	}, &types.OrderInfoRequest{})
}
