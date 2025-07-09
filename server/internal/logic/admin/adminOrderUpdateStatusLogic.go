package admin

import (
  "context"
  "fmt"
  "server/internal/svc"
  "server/internal/types"

  "github.com/zeromicro/go-zero/core/logx"
)

type AdminOrderUpdateStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminOrderUpdateStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminOrderUpdateStatusLogic {
	return &AdminOrderUpdateStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminOrderUpdateStatusLogic) AdminOrderUpdateStatus(req *types.OrderUpdateStatusRequest) (resp *types.OrderUpdateStatusResponse, err error) {
	// 验证状态值
	if req.Status < -1 || req.Status > 2 {
		return nil, fmt.Errorf("无效的状态值: %d", req.Status)
	}

	// 更新订单状态
	err = l.svcCtx.OrderModel.UpdateStatus(l.ctx, uint64(req.OrderId), req.Status)
	if err != nil {
		return nil, err
	}

	return &types.OrderUpdateStatusResponse{}, nil
}
