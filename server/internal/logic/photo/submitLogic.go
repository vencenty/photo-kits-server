package photo

import (
	"context"
	"github.com/zeromicro/x/errors"
	"net/http"
	"photo-kits-server/server/internal/svc"
	"photo-kits-server/server/internal/types"
	"photo-kits-server/server/model"

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

	return nil, errors.New(http.StatusUnauthorized, "出错了")

}
