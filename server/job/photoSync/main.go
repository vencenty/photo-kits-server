package main

import (
	"context"
	"flag"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"path/filepath"
	"photo-kits-server/server/job/photoSync/config"
	syncpkg "photo-kits-server/server/job/photoSync/sync"
	"time"
)

var configFile = flag.String("f", "etc/photo-api.yaml", "the config file")
var syncConfigFile = flag.String("syncConfig", "job/photoSync/etc/syncConfig.yaml", "additional sync config file")

func main() {
	flag.Parse()

	// 尝试查找主配置文件
	mainConfigPath := findConfigFile(*configFile, []string{
		*configFile,
		"./etc/photo-api.yaml",
		"../etc/photo-api.yaml",
		"../../etc/photo-api.yaml",
	})

	// 尝试查找同步配置文件
	syncConfigPath := findConfigFile(*syncConfigFile, []string{
		*syncConfigFile,
		"./job/photoSync/etc/syncConfig.yaml",
		"../photoSync/etc/syncConfig.yaml",
		"../../job/photoSync/etc/syncConfig.yaml",
	})

	logx.Infof("使用主配置文件: %s", mainConfigPath)
	logx.Infof("使用同步配置文件: %s", syncConfigPath)

	// 加载主配置文件
	var c config.Config
	conf.MustLoad(mainConfigPath, &c)

	// 加载同步任务特定配置
	var syncConfig config.SyncConfig
	conf.MustLoad(syncConfigPath, &syncConfig)

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

// findConfigFile 尝试在多个位置查找配置文件
func findConfigFile(userPath string, searchPaths []string) string {
	// 首先检查用户提供的路径
	if _, err := os.Stat(userPath); err == nil {
		return userPath
	}

	// 尝试可执行文件相对路径
	execDir, err := getExecutableDir()
	if err == nil {
		execPath := filepath.Join(execDir, userPath)
		if _, err := os.Stat(execPath); err == nil {
			return execPath
		}
	}

	// 尝试其他搜索路径
	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}

		// 尝试相对于可执行文件的路径
		if execDir != "" {
			execPath := filepath.Join(execDir, path)
			if _, err := os.Stat(execPath); err == nil {
				return execPath
			}
		}
	}

	// 如果找不到，返回原始路径
	return userPath
}

// getExecutableDir 获取当前可执行文件所在目录
func getExecutableDir() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(execPath), nil
}
