package admin

import (
	"net/http"

	"server/internal/logic/admin"
	"server/internal/svc"
	"server/internal/types"
	"server/internal/utils"
)

func AdminOrderDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return utils.WrapHandlerWithRequest(func(w http.ResponseWriter, r *http.Request, req interface{}) (interface{}, error) {
		l := admin.NewAdminOrderDeleteLogic(r.Context(), svcCtx)
		return l.AdminOrderDelete(req.(*types.OrderDeleteRequest))
	}, &types.OrderDeleteRequest{})
}
