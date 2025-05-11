package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"photo-kits-server/server/internal/config"
	"photo-kits-server/server/internal/handler"
	"photo-kits-server/server/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/photo-api.yaml", "the config file")

func main() {
	flag.Parse()

	// 查找配置文件
	configPath := findConfigFile(*configFile, []string{
		*configFile,
		"./etc/photo-api.yaml",
		"../etc/photo-api.yaml",
	})

	logx.Infof("使用配置文件: %s", configPath)

	var c config.Config
	conf.MustLoad(configPath, &c)

	server := rest.MustNewServer(
		c.RestConf,
		rest.WithCors("*"),
		rest.WithCorsHeaders("country,lang"),
	)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
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
