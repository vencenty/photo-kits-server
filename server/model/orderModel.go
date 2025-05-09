package model

import (
	"context"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OrderModel = (*customOrderModel)(nil)

const (
	// 订单待处理
	OrderStatusPending = 0
	// 订单已经锁定
	OrderStatusProcessing = 1
	// 订单已完成
	OrderStatusCompleted = 2
)

type (
	// OrderModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOrderModel.
	OrderModel interface {
		orderModel
		withSession(session sqlx.Session) OrderModel
		FindPendingOrders(ctx context.Context, limit int) ([]*Order, error)
		UpdateStatus(ctx context.Context, id uint64, status int64) error
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

// FindPendingOrders 查找待处理的订单
func (m *customOrderModel) FindPendingOrders(ctx context.Context, limit int) ([]*Order, error) {
	var orders []*Order
	query := fmt.Sprintf("select %s from %s where `status` = ? limit ?", orderRows, m.table)
	err := m.conn.QueryRowsCtx(ctx, &orders, query, OrderStatusPending, limit)
	if err != nil {
		return nil, err
	}
	return orders, nil
}

// UpdateStatus 更新订单状态
func (m *customOrderModel) UpdateStatus(ctx context.Context, id uint64, status int64) error {
	query := fmt.Sprintf("update %s set `status` = ? where `id` = ?", m.table)
	_, err := m.conn.ExecCtx(ctx, query, status, id)
	return err
}
