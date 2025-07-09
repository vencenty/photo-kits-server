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

	// 按照规格分组照片URL
	photosBySpec := make(map[string][]types.PhotoMetadata)
	for _, photo := range photos {
		photosBySpec[photo.Spec] = append(photosBySpec[photo.Spec], types.PhotoMetadata{
			URL:       photo.OriginUrl,
			IsResized: photo.IsResized,
		})
	}

	// 转换为响应格式
	photoList := make([]types.Photo, 0, len(photosBySpec))
	for spec, metadata := range photosBySpec {
		photo := types.Photo{
			Spec:     spec,
			Metadata: metadata,
		}
		photoList = append(photoList, photo)
		logx.Infof("规格 %s 的照片数量: %d", spec, len(metadata))
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
