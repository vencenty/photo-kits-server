package main

import (
	"context"
	"flag"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"photo-kits-server/server/job/photoSync/config"
	syncpkg "photo-kits-server/server/job/photoSync/sync"
	"photo-kits-server/server/model"
	"time"
)

var configFile = flag.String("f", "server/job/photoSync/etc/photoSync.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	logx.MustSetup(c.Log)
	defer logx.Close()

	logx.Info("PhotoSync job starting...")

	// 创建数据库连接
	conn := sqlx.NewMysql(c.MySQL.DataSource)
	photoModel := model.NewPhotoModel(conn)

	// 创建上下文，添加超时控制
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.SyncConfig.Timeout)*time.Second)
	defer cancel()

	// 创建同步器并执行同步
	syncer := syncpkg.NewPhotoSyncer(photoModel, c.SyncConfig)
	if err := syncer.SyncPhotos(ctx); err != nil {
		logx.Errorf("照片同步失败: %v", err)
	}

	logx.Info("PhotoSync job completed")
}
