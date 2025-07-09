package admin

import (
	"context"

	"server/internal/svc"
	"server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminOrderInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminOrderInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminOrderInfoLogic {
	return &AdminOrderInfoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminOrderInfoLogic) AdminOrderInfo(req *types.OrderInfoRequest) (resp *types.OrderInfoResponse, err error) {
	// 根据订单号查询订单
	order, err := l.svcCtx.OrderModel.FindOneByOrderSn(l.ctx, req.OrderSn)
	if err != nil {
		return nil, err
	}

	// 查询订单相关的照片
	photos, err := l.svcCtx.PhotoModel.FindByOrderId(l.ctx, order.Id)
	if err != nil {
		return nil, err
	}

	// 构建照片数据
	photoList := make([]types.Photo, 0, len(photos))
	for _, photo := range photos {
		photoList = append(photoList, types.Photo{
			Spec: photo.Spec,
			Metadata: []types.PhotoMetadata{
				{
					URL:       photo.Url,
					IsResized: photo.IsResized,
				},
			},
		})
	}

	return &types.OrderInfoResponse{
		OrderSn:   order.OrderSn,
		Receiver:  order.Receiver,
		Remark:    order.Remark,
		Status:    order.Status,
		CreatedAt: order.CreatedAt.Format("2006-01-02 15:04:05"),
		Photos:    photoList,
	}, nil
}
