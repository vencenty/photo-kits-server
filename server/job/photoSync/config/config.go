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
	SourcePath string `json:",default=/Users/yanchengtian/Workspace/MyProjects/photo-kits/abc"`
	BackupPath string `json:",default=./data/backup"`
	OutputPath string `json:",default=/Users/yanchengtian/Workspace/MyProjects/photo-kits/abc"`

	// 照片下载重试配置
	DownloadTimeout    int `json:",default=120"` // 初始下载超时时间（秒）
	MaxRetries         int `json:",default=3"`   // 最大重试次数
	RetryBaseDelay     int `json:",default=2"`   // 重试基础延迟时间（秒）
	MaxDownloadTimeout int `json:",default=120"` // 最大下载超时时间（秒）

	// 异步重试配置
	AsyncRetry       bool `json:",default=true"` // 是否启用异步重试
	RetryWorkers     int  `json:",default=5"`    // 重试协程池大小
	RetryChannelSize int  `json:",default=1000"` // 重试队列大小
}
