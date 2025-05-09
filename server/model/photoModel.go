package model

import (
	"context"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ PhotoModel = (*customPhotoModel)(nil)

type (
	// PhotoModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPhotoModel.
	PhotoModel interface {
		photoModel
		withSession(session sqlx.Session) PhotoModel
		DeleteByOrderId(ctx context.Context, orderId uint64) error
		FindByOrderId(ctx context.Context, orderId uint64) ([]*Photo, error)
	}

	customPhotoModel struct {
		*defaultPhotoModel
	}
)

// NewPhotoModel returns a model for the database table.
func NewPhotoModel(conn sqlx.SqlConn) PhotoModel {
	return &customPhotoModel{
		defaultPhotoModel: newPhotoModel(conn),
	}
}

func (m *customPhotoModel) withSession(session sqlx.Session) PhotoModel {
	return NewPhotoModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customPhotoModel) DeleteByOrderId(ctx context.Context, orderId uint64) error {
	query := fmt.Sprintf("delete from %s where `order_id` = ?", m.table)
	_, err := m.conn.ExecCtx(ctx, query, orderId)
	return err
}

// FindByOrderId 查找订单的所有照片
func (m *customPhotoModel) FindByOrderId(ctx context.Context, orderId uint64) ([]*Photo, error) {
	var photos []*Photo
	query := fmt.Sprintf("select %s from %s where `order_id` = ?", photoRows, m.table)
	err := m.conn.QueryRowsCtx(ctx, &photos, query, orderId)
	if err != nil {
		return nil, err
	}
	return photos, nil
}
