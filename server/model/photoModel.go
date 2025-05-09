package model

import (
	"context"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ PhotoModel = (*customPhotoModel)(nil)

const (
	// PhotoStatusPending 照片待下载
	PhotoStatusPending = 0
	// PhotoStatusSuccess 照片下载成功
	PhotoStatusSuccess = 1
	// PhotoStatusFailed 照片下载失败
	PhotoStatusFailed = -1
)

type (
	// PhotoModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPhotoModel.
	PhotoModel interface {
		photoModel
		withSession(session sqlx.Session) PhotoModel
		DeleteByOrderId(ctx context.Context, orderId uint64) error
		FindByOrderId(ctx context.Context, orderId uint64) ([]*Photo, error)
		FindFailedPhotos(ctx context.Context, limit int) ([]*Photo, error)
		UpdateStatus(ctx context.Context, id uint64, status int64, errMsg string) error
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

// FindFailedPhotos 查找下载失败的照片，用于重试
func (m *customPhotoModel) FindFailedPhotos(ctx context.Context, limit int) ([]*Photo, error) {
	var photos []*Photo
	query := fmt.Sprintf("select %s from %s where `status` = ? limit ?", photoRows, m.table)
	err := m.conn.QueryRowsCtx(ctx, &photos, query, PhotoStatusFailed, limit)
	if err != nil {
		return nil, err
	}
	return photos, nil
}

// UpdateStatus 更新照片状态和错误信息
func (m *customPhotoModel) UpdateStatus(ctx context.Context, id uint64, status int64, errMsg string) error {
	query := fmt.Sprintf("update %s set `status` = ?, `error` = ? where `id` = ?", m.table)
	_, err := m.conn.ExecCtx(ctx, query, status, errMsg, id)
	return err
}
