package svc

import (
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"
	"server/internal/config"
	"server/internal/middleware"
	"server/model"
)

type ServiceContext struct {
	Config         config.Config
	DB             sqlx.SqlConn
	CorsMiddleware rest.Middleware
	OrderModel     model.OrderModel
	PhotoModel     model.PhotoModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Database.DataSource)
	return &ServiceContext{
		Config:         c,
		DB:             conn,
		CorsMiddleware: middleware.NewCorsMiddleware().Handle,
		OrderModel:     model.NewOrderModel(conn),
		PhotoModel:     model.NewPhotoModel(conn),
	}
}
