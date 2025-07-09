package admin

import (
	"context"
	"math"

	"server/internal/svc"
	"server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminOrderListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminOrderListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminOrderListLogic {
	return &AdminOrderListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminOrderListLogic) AdminOrderList(req *types.OrderListRequest) (resp *types.OrderListResponse, err error) {
	// 设置默认分页参数
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	// 查询订单列表
	orders, total, err := l.svcCtx.OrderModel.FindOrdersWithPagination(
		l.ctx,
		req.OrderSn,
		req.Receiver,
		req.Remark,
		req.Status,
		req.CreatedAt,
		req.Page,
		req.PageSize,
	)
	if err != nil {
		return nil, err
	}

	// 构建响应数据
	orderItems := make([]types.OrderItem, 0, len(orders))
	for _, order := range orders {
		orderItems = append(orderItems, types.OrderItem{
			ID:        int64(order.Id),
			OrderSn:   order.OrderSn,
			Receiver:  order.Receiver,
			Status:    order.Status,
			CreatedAt: order.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	// 计算总页数
	pages := int64(math.Ceil(float64(total) / float64(req.PageSize)))

	return &types.OrderListResponse{
		List:     orderItems,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Pages:    pages,
	}, nil
}
