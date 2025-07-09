package api

import (
  "net/http"
  "server/internal/logic/api"

  "server/internal/svc"
  "server/internal/types"
  "server/internal/utils"
)

func SubmitHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return utils.WrapHandlerWithRequest(func(w http.ResponseWriter, r *http.Request, req interface{}) (interface{}, error) {
		l := api.NewSubmitLogic(r.Context(), svcCtx)
		return l.Submit(req.(*types.SubmitRequest))
	}, &types.SubmitRequest{})
}
