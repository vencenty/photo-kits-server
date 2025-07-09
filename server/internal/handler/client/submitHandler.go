package client

import (
	"net/http"

	"server/internal/logic/client"
	"server/internal/svc"
	"server/internal/types"
	"server/internal/utils"
)

func SubmitHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return utils.WrapHandlerWithRequest(func(w http.ResponseWriter, r *http.Request, req interface{}) (interface{}, error) {
		l := client.NewSubmitLogic(r.Context(), svcCtx)
		return l.Submit(req.(*types.SubmitRequest))
	}, &types.SubmitRequest{})
}
