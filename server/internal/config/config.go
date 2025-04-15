package config

import "github.com/zeromicro/go-zero/rest"

type Config struct {
	rest.RestConf
	Path     string   `json:"path"`
	Database Database `json:"database"`
}

type Database struct {
	DataSource string `json:"dataSource"`
}
