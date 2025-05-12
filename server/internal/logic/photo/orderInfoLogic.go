package photo

import (
	"context"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/x/errors"
	"photo-kits-server/server/internal/svc"
	"photo-kits-server/server/internal/types"
	"photo-kits-server/server/model"
)

type OrderInfoLogic struct {
	logx.Logger
	ctx        context.Context
	svcCtx     *svc.ServiceContext
	orderModel model.OrderModel
	photoModel model.PhotoModel
}

func NewOrderInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OrderInfoLogic {
	return &OrderInfoLogic{
		Logger:     logx.WithContext(ctx),
		ctx:        ctx,
		svcCtx:     svcCtx,
		orderModel: model.NewOrderModel(svcCtx.DB),
		photoModel: model.NewPhotoModel(svcCtx.DB),
	}
}

func (l *OrderInfoLogic) OrderInfo(req *types.OrderInfoRequest) (resp *types.OrderInfoResponse, err error) {
	logx.Infof("获取订单信息, order_sn: %s", req.OrderSn)

	resp = &types.OrderInfoResponse{}

	// 看下能否找到订单
	order, err := l.orderModel.FindOneByOrderSn(l.ctx, req.OrderSn)
	if err != nil {
		logx.Errorf("查询订单失败, order_sn: %s, error: %v", req.OrderSn, err)
		return resp, errors.New(-1, "订单不存在")
	}

	logx.Infof("订单信息查询成功, order_id: %d, order_sn: %s", order.Id, order.OrderSn)

	// 根据订单信息找到所有的图片
	photos, err := l.photoModel.FindByOrderId(l.ctx, order.Id)
	if err != nil {
		logx.Errorf("查询订单照片失败, order_id: %d, error: %v", order.Id, err)
		return resp, errors.New(-1, "获取照片信息失败")
	}

	logx.Infof("订单照片查询成功, order_id: %d, photo_count: %d", order.Id, len(photos))

	// 填充订单基本信息
	resp.OrderSn = order.OrderSn
	resp.Receiver = order.Receiver
	resp.Remark = order.Remark
	resp.Status = order.Status
	resp.CreatedAt = order.CreatedAt.Format("2006-01-02 15:04:05")

	// 按照规格分组照片URL
	photosBySpec := make(map[string][]string)
	for _, p := range photos {
		photosBySpec[p.Spec] = append(photosBySpec[p.Spec], p.Url)
	}

	// 转换为响应格式
	resp.Photos = make([]types.Photo, 0, len(photosBySpec))
	for spec, urls := range photosBySpec {
		photo := types.Photo{
			Spec: spec,
			Urls: urls,
		}
		resp.Photos = append(resp.Photos, photo)
		logx.Infof("规格 %s 的照片数量: %d", spec, len(urls))
	}

	logx.Infof("订单信息查询完成, order_sn: %s, 共 %d 种规格, 总照片数: %d",
		req.OrderSn, len(resp.Photos), len(photos))

	return resp, nil
}
