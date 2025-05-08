package photo

import (
	"context"
	"database/sql"
	stdErrors "errors"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
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
		order       *model.Order
		result      sql.Result
		totalPhotos int64
		orderId     int64
	)

	order, err = l.orderModel.FindOneByOrderSn(l.ctx, req.OrderSn)
	if err != nil && !stdErrors.Is(err, sqlx.ErrNotFound) {
		return resp, err
	}

	// 如果订单存在，并且订单已经进入处理中状态，那么不允许用户重新上传
	if order != nil && order.Status == model.OrderStatusProcessing {
		return resp, errors.New(-1, "订单已经进入处理流程，无法重新上传图片，如有疑问请联系田田洗照片处理")
	}

	// 没有订单的话创建订单
	if order == nil {
		order = &model.Order{
			OrderSn:   req.OrderSn,
			Receiver:  req.Receiver,
			Remark:    req.Remark,
			Status:    model.OrderStatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if result, err = l.orderModel.Insert(l.ctx, order); err != nil {
			return nil, err
		}
		if orderId, err = result.LastInsertId(); err != nil {
			return nil, err
		}
		order.Id = uint64(orderId)

	} else {
		order.Receiver = req.Receiver
		order.Remark = req.Remark
		order.UpdatedAt = time.Now()

		if err = l.orderModel.Update(l.ctx, order); err != nil {
			return nil, err
		}

		// 删除订单下关联的订单数据
		if err = l.photoModel.DeleteByOrderId(l.ctx, order.Id); err != nil {
			return nil, err
		}
	}

	// 把照片数据关联给订单

	photos := make([]*model.Photo, 0)
	for _, photo := range req.Photos {
		// 添加每个URL对应的照片记录
		for _, url := range photo.Urls {
			if url == "" {
				continue
			}

			p := &model.Photo{
				OrderId:   order.Id,
				Url:       url,
				Size:      photo.Size,
				Unit:      photo.Unit,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			photos = append(photos, p)
			totalPhotos++
		}
	}

	for _, photo := range photos {
		if _, err = l.photoModel.Insert(l.ctx, photo); err != nil {
			return nil, err
		}
	}

	resp = new(types.SubmitResponse)
	resp.Total = totalPhotos

	// 如果订单已经存在，那么删除订单下所有关联的photo
	return resp, nil
}
