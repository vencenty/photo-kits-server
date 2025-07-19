package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jinzhu/copier"
	"server/internal/svc"
	"server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminInfoLogic {
	return &AdminInfoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminInfoLogic) AdminInfo(req *types.AdminInfoRequest) (resp *types.AdminInfoResponse, err error) {
	id, ok := l.ctx.Value("id").(json.Number)
	if !ok {
		return nil, fmt.Errorf("id is not a number")
	}

	i, _ := id.Int64()

	model, err := l.svcCtx.AdminModel.FindOne(l.ctx, i)
	if err != nil {
		return nil, err
	}

	copier.Copy(&resp, model)

	return resp, nil

}
