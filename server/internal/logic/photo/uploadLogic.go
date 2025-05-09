package photo

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/minio/minio-go"
	"github.com/zeromicro/go-zero/core/logx"
	"io"
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
	// 创建 SHA1 哈希对象
	hasher := sha1.New()

	// 创建 TeeReader：会把读取内容同时写入 hasher
	teeReader := io.TeeReader(file, hasher)
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
		teeReader,
		handler.Size,
		minio.PutObjectOptions{ContentType: handler.Header.Get("Content-Type")},
	)
	if err != nil {
		logx.Errorf("minioClientPutObjectError:%v", err)
		return resp, err
	}

	// 返回相应
	return &types.UploadResponse{
		Filename: handler.Filename,
		Size:     handler.Size,
		Sha1:     fileSha1Sum,
		URL:      fmt.Sprintf("%s://%s/%s/%s", l.svcCtx.Config.Minio.Schema, l.svcCtx.Config.Minio.Endpoint, l.svcCtx.Config.Minio.Bucket, objectName),
	}, nil

}
