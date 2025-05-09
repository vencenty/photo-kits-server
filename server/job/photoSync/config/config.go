package config

import (
	"github.com/zeromicro/go-zero/core/logx"
)

// Config 合并主配置和同步任务特定配置
type Config struct {
	Name       string
	Host       string
	Port       int
	Timeout    int
	MaxBytes   int64
	Log        logx.LogConf
	Database   DatabaseConfig
	Minio      MinioConfig
	SyncConfig SyncConfig // 同步任务特有配置
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Datasource string
}

// MinioConfig Minio配置
type MinioConfig struct {
	Schema    string
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
	Bucket    string
}

// SyncConfig 同步任务特有配置
type SyncConfig struct {
	BatchSize  int    `json:",default=100"`
	Timeout    int    `json:",default=3600"` // 同步超时时间（秒）
	SourcePath string `json:",default=./data/photos"`
	BackupPath string `json:",default=./data/backup"`
}
