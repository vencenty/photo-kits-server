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

var configFile = flag.String("f", "server/etc/photo-api.yaml", "the config file")
var syncConfigFile = flag.String("syncConfig", "server/job/photoSync/etc/syncConfig.yaml", "additional sync config file")

func main() {
	flag.Parse()

	// 加载主配置文件
	var c config.Config
	conf.MustLoad(*configFile, &c)

	// 加载同步任务特定配置
	var syncConfig config.SyncConfig
	conf.MustLoad(*syncConfigFile, &syncConfig)

	// 合并配置
	c.SyncConfig = syncConfig

	logx.MustSetup(c.Log)
	defer logx.Close()

	logx.Info("PhotoSync job starting...")

	// 创建数据库连接，使用主配置中的数据库配置
	db := sqlx.NewMysql(c.Database.Datasource)

	// 创建上下文，添加超时控制
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.SyncConfig.Timeout)*time.Second)
	defer cancel()

	// 创建同步器并执行同步
	syncer := syncpkg.NewPhotoSyncer(db, c.SyncConfig)
	if err := syncer.SyncPhotos(ctx); err != nil {
		logx.Errorf("照片同步失败: %v", err)
	}

	logx.Info("PhotoSync job completed")
}
