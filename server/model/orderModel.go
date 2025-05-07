package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ OrderModel = (*customOrderModel)(nil)

const (
	// 订单待处理
	OrderStatusPending = 0
	// 订单已经锁定
	OrderStatusProcessing = 1
)

type (
	// OrderModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOrderModel.
	OrderModel interface {
		orderModel
		withSession(session sqlx.Session) OrderModel
	}

	customOrderModel struct {
		*defaultOrderModel
	}
)

// NewOrderModel returns a model for the database table.
func NewOrderModel(conn sqlx.SqlConn) OrderModel {
	return &customOrderModel{
		defaultOrderModel: newOrderModel(conn),
	}
}

func (m *customOrderModel) withSession(session sqlx.Session) OrderModel {
	return NewOrderModel(sqlx.NewSqlConnFromSession(session))
}
