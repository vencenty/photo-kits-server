package admin

import (
	"net/http"
	"server/internal/logic/admin"
	"server/internal/types"
	"server/internal/utils"

	"server/internal/svc"
)

func AdminInfoHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return utils.WrapHandlerWithRequest(func(w http.ResponseWriter, r *http.Request, req interface{}) (interface{}, error) {
		l := admin.NewAdminInfoLogic(r.Context(), svcCtx)
		return l.AdminInfo(req.(*types.AdminInfoRequest))
	}, &types.AdminInfoRequest{})
}
