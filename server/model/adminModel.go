package model

import (
	"context"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AdminModel = (*customAdminModel)(nil)

type (
	// AdminModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAdminModel.
	AdminModel interface {
		adminModel
		withSession(session sqlx.Session) AdminModel
		FindByAccount(ctx context.Context, account string) (*Admin, error)
	}

	customAdminModel struct {
		*defaultAdminModel
	}
)

// NewAdminModel returns a model for the database table.
func NewAdminModel(conn sqlx.SqlConn) AdminModel {
	return &customAdminModel{
		defaultAdminModel: newAdminModel(conn),
	}
}

func (m *customAdminModel) withSession(session sqlx.Session) AdminModel {
	return NewAdminModel(sqlx.NewSqlConnFromSession(session))
}

// FindByAccount 根据邮箱或手机号查找管理员
func (m *customAdminModel) FindByAccount(ctx context.Context, account string) (*Admin, error) {
	var resp Admin
	query := fmt.Sprintf("select %s from %s where `email` = ? or `mobile` = ? limit 1", adminRows, m.table)
	err := m.conn.QueryRowCtx(ctx, &resp, query, account, account)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
