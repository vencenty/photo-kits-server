package svc

import (
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"photo-kits-server/server/internal/config"
)

type ServiceContext struct {
	Config config.Config
	DB     sqlx.SqlConn
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config: c,
		DB:     sqlx.NewMysql(c.Database.DataSource),
	}

}
