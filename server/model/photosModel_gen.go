// Code generated by goctl. DO NOT EDIT.
// versions:
//  goctl version: 1.8.1

package model

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/builder"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/core/stringx"
)

var (
	photosFieldNames          = builder.RawFieldNames(&Photos{})
	photosRows                = strings.Join(photosFieldNames, ",")
	photosRowsExpectAutoSet   = strings.Join(stringx.Remove(photosFieldNames, "`id`", "`create_at`", "`create_time`", "`created_at`", "`update_at`", "`update_time`", "`updated_at`"), ",")
	photosRowsWithPlaceHolder = strings.Join(stringx.Remove(photosFieldNames, "`id`", "`create_at`", "`create_time`", "`created_at`", "`update_at`", "`update_time`", "`updated_at`"), "=?,") + "=?"
)

type (
	photosModel interface {
		Insert(ctx context.Context, data *Photos) (sql.Result, error)
		FindOne(ctx context.Context, id uint64) (*Photos, error)
		Update(ctx context.Context, data *Photos) error
		Delete(ctx context.Context, id uint64) error
	}

	defaultPhotosModel struct {
		conn  sqlx.SqlConn
		table string
	}

	Photos struct {
		Id        uint64    `db:"id"`
		OrderId   uint64    `db:"order_id"`
		Url       string    `db:"url"`
		Size      int64     `db:"size"`
		Unit      string    `db:"unit"`
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}
)

func newPhotosModel(conn sqlx.SqlConn) *defaultPhotosModel {
	return &defaultPhotosModel{
		conn:  conn,
		table: "`photos`",
	}
}

func (m *defaultPhotosModel) Delete(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("delete from %s where `id` = ?", m.table)
	_, err := m.conn.ExecCtx(ctx, query, id)
	return err
}

func (m *defaultPhotosModel) FindOne(ctx context.Context, id uint64) (*Photos, error) {
	query := fmt.Sprintf("select %s from %s where `id` = ? limit 1", photosRows, m.table)
	var resp Photos
	err := m.conn.QueryRowCtx(ctx, &resp, query, id)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *defaultPhotosModel) Insert(ctx context.Context, data *Photos) (sql.Result, error) {
	query := fmt.Sprintf("insert into %s (%s) values (?, ?, ?, ?)", m.table, photosRowsExpectAutoSet)
	ret, err := m.conn.ExecCtx(ctx, query, data.OrderId, data.Url, data.Size, data.Unit)
	return ret, err
}

func (m *defaultPhotosModel) Update(ctx context.Context, data *Photos) error {
	query := fmt.Sprintf("update %s set %s where `id` = ?", m.table, photosRowsWithPlaceHolder)
	_, err := m.conn.ExecCtx(ctx, query, data.OrderId, data.Url, data.Size, data.Unit, data.Id)
	return err
}

func (m *defaultPhotosModel) tableName() string {
	return m.table
}
