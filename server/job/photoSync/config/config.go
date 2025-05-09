package config

import (
	"github.com/zeromicro/go-zero/core/logx"
)

type Config struct {
	Log        logx.LogConf
	MySQL      MySQLConfig
	SyncConfig SyncConfig
}

type MySQLConfig struct {
	DataSource string
}

type SyncConfig struct {
	BatchSize  int    `json:",default=100"`
	Timeout    int    `json:",default=3600"` // 同步超时时间（秒）
	SourcePath string `json:",default=./data/photos"`
	BackupPath string `json:",default=./data/backup"`
}
