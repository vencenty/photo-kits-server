package admin

import (
	"context"

	"server/internal/svc"
	"server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminOrderDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminOrderDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminOrderDeleteLogic {
	return &AdminOrderDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminOrderDeleteLogic) AdminOrderDelete(req *types.OrderDeleteRequest) (resp *types.OrderDeleteResponse, err error) {
	if len(req.OrderIds) == 0 {
		return &types.OrderDeleteResponse{Total: 0}, nil
	}

	// 删除订单
	deletedCount, err := l.svcCtx.OrderModel.DeleteOrdersByIds(l.ctx, req.OrderIds)
	if err != nil {
		return nil, err
	}

	// 删除相关的照片
	err = l.svcCtx.PhotoModel.DeleteByOrderIds(l.ctx, req.OrderIds)
	if err != nil {
		logx.Errorf("删除订单照片失败: %v", err)
		// 这里不返回错误，因为订单已经删除了
	}

	return &types.OrderDeleteResponse{
		Total: deletedCount,
	}, nil
}
