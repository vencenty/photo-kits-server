package model

import (
	"context"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OrderModel = (*customOrderModel)(nil)

const (
	// 失败（可能部分失败）
	OrderStatusFailed = -1
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
		FindProcessingOrders(ctx context.Context, limit int) ([]*Order, error)
		UpdateStatus(ctx context.Context, id uint64, status int64) error
		GetAndLockPendingOrder(ctx context.Context) (*Order, error)
		FindOrdersWithPagination(ctx context.Context, orderSn, receiver, remark string, status int64, createdAt string, page, pageSize int64) ([]*Order, int64, error)
		DeleteOrdersByIds(ctx context.Context, orderIds []int64) (int64, error)
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

// FindProcessingOrders 查找处理中的订单
func (m *customOrderModel) FindProcessingOrders(ctx context.Context, limit int) ([]*Order, error) {
	var orders []*Order
	query := fmt.Sprintf("select %s from %s where `status` = ? limit ?", orderRows, m.table)
	err := m.conn.QueryRowsCtx(ctx, &orders, query, OrderStatusProcessing, limit)
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

// GetAndLockPendingOrder 原子性获取并锁定一个待处理订单
func (m *customOrderModel) GetAndLockPendingOrder(ctx context.Context) (*Order, error) {
	// 使用MySQL的原子更新操作来实现锁定
	// 直接更新一行并返回更新前的信息
	updateQuery := fmt.Sprintf(`
		UPDATE %s 
		SET status = ? 
		WHERE id = (
			SELECT id FROM (
				SELECT id FROM %s 
				WHERE status IN (?, ?) 
				ORDER BY created_at ASC 
				LIMIT 1
			) AS tmp
		)
	`, m.table, m.table)

	result, err := m.conn.ExecCtx(ctx, updateQuery, OrderStatusProcessing, OrderStatusPending, OrderStatusFailed)
	if err != nil {
		return nil, err
	}

	// 检查是否有行被更新
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		// 没有待处理的订单
		return nil, fmt.Errorf("record not found")
	}

	// 查询刚刚更新的订单
	findQuery := fmt.Sprintf("select %s from %s where status = ? order by updated_at desc limit 1", orderRows, m.table)
	var order Order
	err = m.conn.QueryRowCtx(ctx, &order, findQuery, OrderStatusProcessing)
	if err != nil {
		return nil, err
	}

	return &order, nil
}

// FindOrdersWithPagination 分页查询订单列表
func (m *customOrderModel) FindOrdersWithPagination(ctx context.Context, orderSn, receiver, remark string, status int64, createdAt string, page, pageSize int64) ([]*Order, int64, error) {
	// 构建查询条件
	var conditions []string
	var args []interface{}

	if orderSn != "" {
		conditions = append(conditions, "`order_sn` LIKE ?")
		args = append(args, "%"+orderSn+"%")
	}

	if receiver != "" {
		conditions = append(conditions, "`receiver` LIKE ?")
		args = append(args, "%"+receiver+"%")
	}

	if remark != "" {
		conditions = append(conditions, "`remark` LIKE ?")
		args = append(args, "%"+remark+"%")
	}

	if status > 0 {
		conditions = append(conditions, "`status` = ?")
		args = append(args, status)
	}

	if createdAt != "" {
		conditions = append(conditions, "DATE(`created_at`) = ?")
		args = append(args, createdAt)
	}

	// 构建WHERE子句
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + fmt.Sprintf("%s", conditions[0])
		for i := 1; i < len(conditions); i++ {
			whereClause += " AND " + conditions[i]
		}
	}

	// 查询总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s%s", m.table, whereClause)
	var total int64
	err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, err
	}

	// 如果总数为0，直接返回
	if total == 0 {
		return []*Order{}, 0, nil
	}

	// 计算分页参数
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	// 查询数据
	dataQuery := fmt.Sprintf("SELECT %s FROM %s%s ORDER BY `created_at` DESC LIMIT ? OFFSET ?", orderRows, m.table, whereClause)
	args = append(args, pageSize, offset)

	var orders []*Order
	err = m.conn.QueryRowsCtx(ctx, &orders, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

// DeleteOrdersByIds 批量删除订单
func (m *customOrderModel) DeleteOrdersByIds(ctx context.Context, orderIds []int64) (int64, error) {
	if len(orderIds) == 0 {
		return 0, nil
	}

	// 构建IN子句的占位符
	placeholders := make([]string, len(orderIds))
	args := make([]interface{}, len(orderIds))
	for i, id := range orderIds {
		placeholders[i] = "?"
		args[i] = id
	}

	inClause := ""
	for i, placeholder := range placeholders {
		if i == 0 {
			inClause = placeholder
		} else {
			inClause += "," + placeholder
		}
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE `id` IN (%s)", m.table, inClause)

	result, err := m.conn.ExecCtx(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rowsAffected, nil
}
