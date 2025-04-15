package logic

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/minio/minio-go"
	"github.com/zeromicro/go-zero/core/logx"
	"io"
	"net/http"
	"photo-kits-server/server/internal/svc"
	"photo-kits-server/server/internal/types"
)

type UploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	r      *http.Request
}

func NewUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext, r *http.Request) *UploadLogic {
	return &UploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
		r:      r,
	}
}

func (l *UploadLogic) Upload() (resp *types.UploadResponse, err error) {

	var (
	//result int64
	)

	file, handler, err := l.r.FormFile("file1")

	if err != nil {
		logx.Errorf("GetFileError:%v", err)
		return nil, nil
	}

	// 创建 SHA1 哈希对象
	hasher := sha1.New()

	// 创建 TeeReader：会把读取内容同时写入 hasher
	teeReader := io.TeeReader(file, hasher)
	sha1Bytes := hasher.Sum(nil)
	fileSha1Sum := hex.EncodeToString(sha1Bytes)

	// 初始化客户端
	minioClient, err := minio.New(l.svcCtx.Config.Minio.Endpoint,
		l.svcCtx.Config.Minio.AccessKey,
		l.svcCtx.Config.Minio.SecretKey,
		l.svcCtx.Config.Minio.UseSSL,
	)
	if err != nil {
		logx.Errorf("minioClientInitError:%v", err)
		return resp, nil
	}

	// 写入bucket
	_, err = minioClient.PutObject(
		l.svcCtx.Config.Minio.Bucket,
		fileSha1Sum,
		teeReader,
		handler.Size,
		minio.PutObjectOptions{ContentType: handler.Header.Get("Content-Type")},
	)
	if err != nil {
		logx.Errorf("minioClientPutObjectError:%v", err)
		return resp, nil
	}

	// 返回相应
	return &types.UploadResponse{
		Filename: handler.Filename,
		Size:     handler.Size,
		Sha1:     fileSha1Sum,
		URL:      fmt.Sprintf("%s://%s/%s/%s", l.svcCtx.Config.Minio.Schema, l.svcCtx.Config.Minio.Endpoint, l.svcCtx.Config.Minio.Bucket, fileSha1Sum),
	}, nil

}
