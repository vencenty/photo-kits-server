package photo

import (
	"context"

	"github.com/zeromicro/x/errors"
	"photo-kits-server/server/internal/svc"
	"photo-kits-server/server/internal/types"
	"photo-kits-server/server/model"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateOrderStatusLogic struct {
	logx.Logger
	ctx        context.Context
	svcCtx     *svc.ServiceContext
	orderModel model.OrderModel
}

func NewUpdateOrderStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateOrderStatusLogic {
	return &UpdateOrderStatusLogic{
		Logger:     logx.WithContext(ctx),
		ctx:        ctx,
		svcCtx:     svcCtx,
		orderModel: model.NewOrderModel(svcCtx.DB),
	}
}

func (l *UpdateOrderStatusLogic) UpdateOrderStatus(req *types.OrderUpdateStatusRequest) (resp *types.OrderUpdateStatusResponse, err error) {
	logx.Infof("更新订单状态, 订单ID: %d, 新状态: %d", req.OrderId, req.Status)

	resp = &types.OrderUpdateStatusResponse{}

	// 验证状态值
	if req.Status < 0 || req.Status > 2 {
		logx.Errorf("无效的订单状态: %d", req.Status)
		return resp, errors.New(-1, "无效的订单状态")
	}

	// 先检查订单是否存在
	order, err := l.orderModel.FindOne(l.ctx, uint64(req.OrderId))
	if err != nil {
		logx.Errorf("查询订单失败, 订单ID: %d, error: %v", req.OrderId, err)
		return resp, errors.New(-1, "订单不存在")
	}

	logx.Infof("订单查询成功, 订单ID: %d, 当前状态: %d", order.Id, order.Status)

	// 更新订单状态
	err = l.orderModel.UpdateStatus(l.ctx, uint64(req.OrderId), req.Status)
	if err != nil {
		logx.Errorf("更新订单状态失败, 订单ID: %d, 新状态: %d, error: %v", req.OrderId, req.Status, err)
		return resp, errors.New(-1, "更新订单状态失败")
	}

	logx.Infof("订单状态更新成功, 订单ID: %d, 从状态 %d 更新为 %d", req.OrderId, order.Status, req.Status)

	return resp, nil
}
