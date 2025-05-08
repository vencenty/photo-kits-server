package photo

import (
	"context"
	"github.com/zeromicro/x/errors"
	"photo-kits-server/server/internal/svc"
	"photo-kits-server/server/internal/types"
	"photo-kits-server/server/model"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

type SubmitLogic struct {
	logx.Logger
	ctx        context.Context
	svcCtx     *svc.ServiceContext
	orderModel model.OrderModel
	photoModel model.PhotoModel
}

func NewSubmitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubmitLogic {
	return &SubmitLogic{
		Logger:     logx.WithContext(ctx),
		ctx:        ctx,
		svcCtx:     svcCtx,
		orderModel: model.NewOrderModel(svcCtx.DB),
		photoModel: model.NewPhotoModel(svcCtx.DB),
	}
}

func (l *SubmitLogic) Submit(req *types.SubmitRequest) (resp *types.SubmitResponse, err error) {

	var (
		order *model.Order
	)

	order, err = l.orderModel.FindOneByOrderSn(l.ctx, req.OrderSn)
	if err != nil {
		return resp, err
	}

	// 如果订单存在，并且订单已经进入处理中状态，那么不允许用户重新上传
	if order != nil && order.Status == model.OrderStatusProcessing {
		return resp, errors.New(-1, "订单已经进入处理流程，无法重新上传图片，如有疑问请联系田田洗照片处理")
	}

	// 没有订单的时候，创建订单，然后关联照片数据
	if order == nil {
		order = &model.Order{
			OrderSn:   req.OrderSn,
			Receiver:  req.Receiver,
			Remark:    req.Remark,
			Status:    0, // 未处理状态
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	} else {

	}

	// 检查订单是否存在，不存在则创建
	order, err := s.photoRepo.GetOrderByOrderSN(req.OrderSn)
	if err != nil {
		// 创建新订单
		order = &model.Order{
			OrderSN:   req.OrderSn,
			Receiver:  req.Receiver,
			Remark:    req.Remark,
			Status:    0, // 未处理状态
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := s.photoRepo.CreateOrder(order); err != nil {
			return nil, errors.New("创建订单失败: " + err.Error())
		}
	} else {
		// 先删除该订单关联的所有照片
		if err = s.photoRepo.DeletePhotosByOrderID(order.ID); err != nil {
			return nil, errors.New("删除旧照片记录失败: " + err.Error())
		}

		// 如果订单存在，更新收货人姓名和备注
		if order.Receiver != req.Receiver || order.Remark != req.Remark {
			order.Receiver = req.Receiver
			order.Remark = req.Remark
			order.UpdatedAt = time.Now()
			if err := s.photoRepo.UpdateOrder(order); err != nil {
				return nil, errors.New("更新订单失败: " + err.Error())
			}
		}
	}

	// 处理照片数据
	var photos []*model.Photo
	totalPhotos := 0

	for _, photo := range req.Photos {
		// 添加每个URL对应的照片记录
		for _, url := range photo.URLs {
			if url == "" {
				continue
			}

			photoModel := &model.Photo{
				OrderID:   order.ID,
				URL:       url,
				Size:      photo.Size,
				Unit:      photo.Unit,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			photos = append(photos, photoModel)
			totalPhotos++
		}
	}

	// 保存照片记录
	if len(photos) > 0 {

		if err = s.photoRepo.DeletePhotosByOrderID(order.ID); err != nil {
			return nil, errors.New("删除旧照片记录失败: " + err.Error())
		}
		if err := s.photoRepo.CreatePhotos(photos); err != nil {
			return nil, errors.New("保存照片记录失败: " + err.Error())
		}
	}

	return &model.PhotoUploadResponse{
		Success:     true,
		TotalPhotos: totalPhotos,
		Message:     "照片上传成功",
	}, nil

	resp = new(types.SubmitResponse)
	resp.Total = 100

	// 如果订单已经存在，那么删除订单下所有关联的photo
	return resp, nil

}
