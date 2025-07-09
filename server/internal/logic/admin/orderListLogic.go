package admin

import (
	"context"
	"math"

	"server/internal/svc"
	"server/internal/types"
	"server/model"

	"github.com/zeromicro/go-zero/core/logx"
)

type OrderListLogic struct {
	logx.Logger
	ctx        context.Context
	svcCtx     *svc.ServiceContext
	orderModel model.OrderModel
}

func NewOrderListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OrderListLogic {
	return &OrderListLogic{
		Logger:     logx.WithContext(ctx),
		ctx:        ctx,
		svcCtx:     svcCtx,
		orderModel: model.NewOrderModel(svcCtx.DB),
	}
}

func (l *OrderListLogic) OrderList(req *types.OrderListRequest) (resp *types.OrderListResponse, err error) {
	logx.Infof("获取订单列表, 参数: %+v", req)

	resp = &types.OrderListResponse{}

	// 设置默认分页参数
	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	// 调用Model查询数据
	orders, total, err := l.orderModel.FindOrdersWithPagination(
		l.ctx,
		req.OrderSn,
		req.Receiver,
		req.Remark,
		req.Status,
		req.CreatedAt,
		page,
		pageSize,
	)
	if err != nil {
		logx.Errorf("查询订单列表失败, error: %v", err)
		return resp, err
	}

	logx.Infof("订单列表查询成功, 总数: %d, 当前页: %d, 每页: %d", total, page, pageSize)

	// 转换数据格式
	resp.List = make([]types.OrderItem, 0, len(orders))
	for _, order := range orders {
		item := types.OrderItem{
			ID:        int64(order.Id),
			OrderSn:   order.OrderSn,
			Receiver:  order.Receiver,
			Status:    order.Status,
			CreatedAt: order.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		resp.List = append(resp.List, item)
	}

	// 设置分页信息
	resp.Total = total
	resp.Page = page
	resp.PageSize = pageSize
	resp.Pages = int64(math.Ceil(float64(total) / float64(pageSize)))

	logx.Infof("订单列表查询完成, 返回 %d 条记录", len(resp.List))

	return resp, nil
}
