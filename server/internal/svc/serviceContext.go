package svc

import (
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"
	"photo-kits-server/server/internal/config"
	"photo-kits-server/server/internal/middleware"
)

type ServiceContext struct {
	Config         config.Config
	DB             sqlx.SqlConn
	CorsMiddleware rest.Middleware
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config:         c,
		DB:             sqlx.NewMysql(c.Database.DataSource),
		CorsMiddleware: middleware.NewCorsMiddleware().Handle,
	}

}
