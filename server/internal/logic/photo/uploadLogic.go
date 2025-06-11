package photo

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/minio/minio-go"
	"github.com/zeromicro/go-zero/core/logx"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"photo-kits-server/server/internal/svc"
	"photo-kits-server/server/internal/types"
	"strings"
)

type UploadLogic struct {
	logx.Logger
	ctx            context.Context
	svcCtx         *svc.ServiceContext
	request        *http.Request
	responseWriter http.ResponseWriter
}

func NewUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext, r *http.Request, w http.ResponseWriter) *UploadLogic {
	return &UploadLogic{
		Logger:         logx.WithContext(ctx),
		ctx:            ctx,
		svcCtx:         svcCtx,
		request:        r,
		responseWriter: w,
	}
}

func (l *UploadLogic) Upload() (resp *types.UploadResponse, err error) {
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

	// 初始化客户端
	minioClient, err := minio.New(l.svcCtx.Config.Minio.Endpoint,
		l.svcCtx.Config.Minio.AccessKey,
		l.svcCtx.Config.Minio.SecretKey,
		l.svcCtx.Config.Minio.UseSSL,
	)
	if err != nil {
		logx.Errorf("minioClientInitError:%v", err)
		return resp, err
	}

	// 生成文件名：原始文件名_{sha1}.文件后缀
	nameWithoutExt := strings.TrimSuffix(handler.Filename, ext)
	objectName := fmt.Sprintf("%s_%s%s", nameWithoutExt, fileSha1Sum, ext)

	logx.Infof("上传文件: %s -> %s", handler.Filename, objectName)

	// 写入 bucket
	_, err = minioClient.PutObject(
		l.svcCtx.Config.Minio.Bucket,
		objectName,
		bytes.NewReader(fileContent),
		int64(len(fileContent)),
		minio.PutObjectOptions{ContentType: handler.Header.Get("Content-Type")},
	)
	if err != nil {
		logx.Errorf("minioClientPutObjectError:%v", err)
		return resp, err
	}

	// 返回响应
	return &types.UploadResponse{
		Filename: handler.Filename,
		Size:     handler.Size,
		Sha1:     fileSha1Sum,
		URL:      fmt.Sprintf("%s://%s/%s/%s", l.svcCtx.Config.Minio.Schema, l.svcCtx.Config.Minio.Endpoint, l.svcCtx.Config.Minio.Bucket, objectName),
	}, nil
}
