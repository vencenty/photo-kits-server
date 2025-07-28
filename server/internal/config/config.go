package config

import "github.com/zeromicro/go-zero/rest"

type Config struct {
	rest.RestConf
	Database  Database  `json:"database"`
	Minio     Minio     `json:"minio"`
	AliyunOSS AliyunOSS `json:"aliyunOSS"`
	AliyunCDN AliyunCDN `json:"aliyunCDN"`
	PushDeer  PushDeer  `json:"pushDeer"`
	Auth      Auth      `json:"auth"`
}
type Auth struct { // JWT 认证需要的密钥和过期时间配置
	AccessSecret string
	AccessExpire int64
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

type AliyunCDN struct {
	Schema   string `json:"schema"`
	EndPoint string `json:"endPoint"`
}

type PushDeer struct {
	Keys []string `json:"keys"`
}

type AliyunOSS struct {
	Endpoint        string `json:"endpoint"`
	AccessKeyId     string `json:"accessKeyId"`
	AccessKeySecret string `json:"accessKeySecret"`
	BucketName      string `json:"bucketName"`
}
