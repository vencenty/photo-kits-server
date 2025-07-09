package admin

import (
	"context"

	"server/internal/svc"
	"server/internal/types"
	"server/model"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteOrderLogic struct {
	logx.Logger
	ctx        context.Context
	svcCtx     *svc.ServiceContext
	orderModel model.OrderModel
	photoModel model.PhotoModel
}

func NewDeleteOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteOrderLogic {
	return &DeleteOrderLogic{
		Logger:     logx.WithContext(ctx),
		ctx:        ctx,
		svcCtx:     svcCtx,
		orderModel: model.NewOrderModel(svcCtx.DB),
		photoModel: model.NewPhotoModel(svcCtx.DB),
	}
}

func (l *DeleteOrderLogic) DeleteOrder(req *types.OrderDeleteRequest) (resp *types.OrderDeleteResponse, err error) {
	logx.Infof("删除订单, 订单IDs: %v", req.OrderIds)

	resp = &types.OrderDeleteResponse{}

	if len(req.OrderIds) == 0 {
		logx.Info("没有要删除的订单")
		resp.Total = 0
		return resp, nil
	}

	// 首先删除所有相关的照片
	err = l.photoModel.DeleteByOrderIds(l.ctx, req.OrderIds)
	if err != nil {
		logx.Errorf("删除订单关联照片失败, orderIds: %v, error: %v", req.OrderIds, err)
		return resp, err
	}

	logx.Infof("成功删除订单关联照片, orderIds: %v", req.OrderIds)

	// 然后删除订单
	deletedCount, err := l.orderModel.DeleteOrdersByIds(l.ctx, req.OrderIds)
	if err != nil {
		logx.Errorf("删除订单失败, orderIds: %v, error: %v", req.OrderIds, err)
		return resp, err
	}

	logx.Infof("删除订单成功, 删除数量: %d, orderIds: %v", deletedCount, req.OrderIds)

	resp.Total = deletedCount

	return resp, nil
}
