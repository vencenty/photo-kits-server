package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ PhotoModel = (*customPhotoModel)(nil)

type (
	// PhotoModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPhotoModel.
	PhotoModel interface {
		photoModel
		withSession(session sqlx.Session) PhotoModel
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
