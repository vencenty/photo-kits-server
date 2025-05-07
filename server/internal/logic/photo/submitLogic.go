package photo

import (
	"context"
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

func (l *SubmitLogic) Submit(req *types.SubmitRequest) error {

	var (
		order *model.Order
		err   error
	)

	order, err = l.orderModel.FindOneByOrderSn(l.ctx, req.OrderSn)
	if err != nil {
		l.Logger.Errorf("订单[%s]不存在", req.OrderSn)
		return err
	}
	_ = order

	// 检查订单是否已经存在了

	return nil
}
