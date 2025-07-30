package api

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/zeromicro/go-zero/core/logx"

	"server/internal/svc"
	"server/internal/types"
)

type UploadOssLogic struct {
	logx.Logger
	ctx            context.Context
	svcCtx         *svc.ServiceContext
	request        *http.Request
	responseWriter http.ResponseWriter
}

func NewUploadOssLogic(ctx context.Context, svcCtx *svc.ServiceContext, r *http.Request, w http.ResponseWriter) *UploadOssLogic {
	return &UploadOssLogic{
		Logger:         logx.WithContext(ctx),
		ctx:            ctx,
		svcCtx:         svcCtx,
		request:        r,
		responseWriter: w,
	}
}

func (l *UploadOssLogic) UploadOss() (resp *types.UploadResponse, err error) {
	file, handler, err := l.request.FormFile("file")
	if err != nil {
		logx.Errorf("GetFileError:%v", err)
		return resp, err
	}
	defer file.Close()

	// 读取文件内容
	fileContent, err := ioutil.ReadAll(file)
	if err != nil {
		logx.Errorf("ReadFileError:%v", err)
		return resp, err
	}

	// 计算SHA1哈希值
	hasher := sha1.New()
	hasher.Write(fileContent)
	sha1Bytes := hasher.Sum(nil)
	fileSha1Sum := hex.EncodeToString(sha1Bytes)

	// 获取文件扩展名
	ext := filepath.Ext(handler.Filename)
	if ext == "" {
		// 如果没有扩展名，尝试从Content-Type中获取
		contentType := handler.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "image/") {
			ext = "." + strings.TrimPrefix(contentType, "image/")
		}
	}

	// 初始化阿里云OSS客户端
	client, err := oss.New(l.svcCtx.Config.AliyunOSS.Endpoint,
		l.svcCtx.Config.AliyunOSS.AccessKeyId,
		l.svcCtx.Config.AliyunOSS.AccessKeySecret)
	if err != nil {
		logx.Errorf("ossClientInitError:%v", err)
		return resp, err
	}

	// 获取存储空间
	bucket, err := client.Bucket(l.svcCtx.Config.AliyunOSS.BucketName)
	if err != nil {
		logx.Errorf("ossBucketError:%v", err)
		return resp, err
	}

	// 生成文件名：原始文件名_{sha1}.文件后缀
	nameWithoutExt := strings.TrimSuffix(handler.Filename, ext)
	objectName := fmt.Sprintf("%s_%s%s", nameWithoutExt, fileSha1Sum, ext)

	logx.Infof("上传文件到OSS: %s -> %s", handler.Filename, objectName)

	// 上传文件到OSS
	err = bucket.PutObject(objectName, bytes.NewReader(fileContent),
		oss.ContentType(handler.Header.Get("Content-Type")))
	if err != nil {
		logx.Errorf("ossPutObjectError:%v", err)
		return resp, err
	}

	// 返回响应
	return &types.UploadResponse{
		Filename: handler.Filename,
		Size:     handler.Size,
		Sha1:     fileSha1Sum,
		//URL:      fmt.Sprintf("%s://%s/%s/%s", l.svcCtx.Config.AliyunOSS.Schema, l.svcCtx.Config.AliyunOSS.CDNDomain, l.svcCtx.Config.AliyunOSS.BucketName, objectName),
		URL: fmt.Sprintf("%s://%s/%s", l.svcCtx.Config.AliyunOSS.Schema, l.svcCtx.Config.AliyunOSS.CDNDomain, objectName),
	}, nil
}
