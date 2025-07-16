package admin

import (
	"net/http"
	"server/internal/utils"

	"server/internal/logic/admin"
	"server/internal/svc"
	"server/internal/types"
)

func AdminLoginHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return utils.WrapHandlerWithRequest(func(w http.ResponseWriter, r *http.Request, req interface{}) (interface{}, error) {
		l := admin.NewAdminLoginLogic(r.Context(), svcCtx)
		return l.AdminLogin(req.(*types.AdminLoginRequest))
	}, &types.AdminLoginRequest{})
}
