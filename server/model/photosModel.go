package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ PhotosModel = (*customPhotosModel)(nil)

type (
	// PhotosModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPhotosModel.
	PhotosModel interface {
		photosModel
		withSession(session sqlx.Session) PhotosModel
	}

	customPhotosModel struct {
		*defaultPhotosModel
	}
)

// NewPhotosModel returns a model for the database table.
func NewPhotosModel(conn sqlx.SqlConn) PhotosModel {
	return &customPhotosModel{
		defaultPhotosModel: newPhotosModel(conn),
	}
}

func (m *customPhotosModel) withSession(session sqlx.Session) PhotosModel {
	return NewPhotosModel(sqlx.NewSqlConnFromSession(session))
}
