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

	// 在 Minio 中查找是否已存在相同 SHA1 的文件
	// 首先尝试直接用 SHA1 作为文件名查找
	objectName := fileSha1Sum + ext
	found := false
	
	_, err = minioClient.StatObject(l.svcCtx.Config.Minio.Bucket, objectName, minio.StatObjectOptions{})
	if err == nil {
		// 对象已存在，直接返回
		found = true
		logx.Infof("文件已存在，SHA1: %s", fileSha1Sum)
	} else {
		// 如果直接查找不到，尝试在 bucket 中列出所有对象，查找包含此 SHA1 的文件
		// 这种情况可能是因为之前上传时使用了带时间戳的文件名
		doneCh := make(chan struct{})
		defer close(doneCh)
		
		for objInfo := range minioClient.ListObjects(l.svcCtx.Config.Minio.Bucket, fileSha1Sum, true, doneCh) {
			if objInfo.Err != nil {
				continue
			}
			
			// 如果找到的对象名包含该 SHA1，则认为文件已存在
			if strings.Contains(objInfo.Key, fileSha1Sum) {
				objectName = objInfo.Key
				found = true
				logx.Infof("找到已存在的文件，SHA1: %s, Key: %s", fileSha1Sum, objInfo.Key)
				break
			}
		}
	}

	// 如果文件已存在，直接返回 URL
	if found {
		return &types.UploadResponse{
			Filename: handler.Filename,
			Size:     handler.Size,
			Sha1:     fileSha1Sum,
			URL:      fmt.Sprintf("%s://%s/%s/%s", l.svcCtx.Config.Minio.Schema, l.svcCtx.Config.Minio.Endpoint, l.svcCtx.Config.Minio.Bucket, objectName),
		}, nil
	}

	// 文件不存在，使用 SHA1 + 时间戳 + 扩展名作为文件名
	timestamp := time.Now().UnixNano()
	objectName = fmt.Sprintf("%s_%d%s", fileSha1Sum, timestamp, ext)

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
