package config

import "github.com/zeromicro/go-zero/rest"

type Config struct {
	rest.RestConf
	Database Database `json:"database"`
	Minio    Minio    `json:"minio"`
}

type Database struct {
	DataSource string `json:"dataSource"`
}

type Minio struct {
	Schema    string `json:"schema"`
	Endpoint  string `json:"endpoint"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	UseSSL    bool   `json:"useSSL"`
	Bucket    string `json:"bucket"`
}
