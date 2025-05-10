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
	"time"
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

	// 使用SHA1哈希值加上原始扩展名作为文件名
	objectName := fileSha1Sum
	if ext != "" {
		objectName = fileSha1Sum + ext
	}

	// 为确保文件名唯一，可以添加时间戳
	timestamp := time.Now().UnixNano()
	objectName = fmt.Sprintf("%s_%d%s", fileSha1Sum, timestamp, ext)

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

	// 写入bucket
	_, err = minioClient.PutObject(
		l.svcCtx.Config.Minio.Bucket,
		objectName,
		bytes.NewReader(fileContent), // 使用bytes.NewReader替代io.NewReader
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
