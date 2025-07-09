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

	// 验证订单ID数组
	if len(req.OrderId) == 0 {
		return nil, fmt.Errorf("订单ID列表不能为空")
	}

	var total int64
	// 批量更新订单状态
	for _, orderId := range req.OrderId {
		err = l.svcCtx.OrderModel.UpdateStatus(l.ctx, uint64(orderId), req.Status)
		if err != nil {
			logx.Errorf("更新订单状态失败, 订单ID: %d, 状态: %d, 错误: %v", orderId, req.Status, err)
			return nil, fmt.Errorf("更新订单状态失败: 订单ID %d, %v", orderId, err)
		}
		logx.Infof("订单状态更新成功, 订单ID: %d, 新状态: %d", orderId, req.Status)
		total++

	}

	return &types.OrderUpdateStatusResponse{
		Total: total,
	}, nil
}
