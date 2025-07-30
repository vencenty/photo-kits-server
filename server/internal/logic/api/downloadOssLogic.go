package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/zeromicro/go-zero/core/logx"

	"server/internal/svc"
	"server/internal/types"
)

type DownloadOssLogic struct {
	logx.Logger
	ctx            context.Context
	svcCtx         *svc.ServiceContext
	request        *http.Request
	responseWriter http.ResponseWriter
}

func NewDownloadOssLogic(ctx context.Context, svcCtx *svc.ServiceContext, r *http.Request, w http.ResponseWriter) *DownloadOssLogic {
	return &DownloadOssLogic{
		Logger:         logx.WithContext(ctx),
		ctx:            ctx,
		svcCtx:         svcCtx,
		request:        r,
		responseWriter: w,
	}
}

func (l *DownloadOssLogic) DownloadOss(req *types.DownloadRequest) error {
	// 参数验证
	if req.Filename == "" {
		l.responseWriter.WriteHeader(http.StatusBadRequest)
		l.responseWriter.Write([]byte("filename parameter is required"))
		return nil
	}

	// 初始化阿里云OSS客户端
	client, err := oss.New(l.svcCtx.Config.AliyunOSS.Endpoint,
		l.svcCtx.Config.AliyunOSS.AccessKeyId,
		l.svcCtx.Config.AliyunOSS.AccessKeySecret)
	if err != nil {
		logx.Errorf("ossClientInitError:%v", err)
		l.responseWriter.WriteHeader(http.StatusInternalServerError)
		l.responseWriter.Write([]byte("Internal server error"))
		return nil
	}

	// 获取存储空间
	bucket, err := client.Bucket(l.svcCtx.Config.AliyunOSS.BucketName)
	if err != nil {
		logx.Errorf("ossBucketError:%v", err)
		l.responseWriter.WriteHeader(http.StatusInternalServerError)
		l.responseWriter.Write([]byte("Internal server error"))
		return nil
	}

	// 检查文件是否存在
	exists, err := bucket.IsObjectExist(req.Filename)
	if err != nil {
		logx.Errorf("ossCheckObjectExistError:%v", err)
		l.responseWriter.WriteHeader(http.StatusInternalServerError)
		l.responseWriter.Write([]byte("Internal server error"))
		return nil
	}

	if !exists {
		l.responseWriter.WriteHeader(http.StatusNotFound)
		l.responseWriter.Write([]byte("File not found"))
		return nil
	}

	// 获取文件对象
	reader, err := bucket.GetObject(req.Filename)
	if err != nil {
		logx.Errorf("ossGetObjectError:%v", err)
		l.responseWriter.WriteHeader(http.StatusInternalServerError)
		l.responseWriter.Write([]byte("Internal server error"))
		return nil
	}
	defer reader.Close()

	// 获取文件信息用于设置响应头
	props, err := bucket.GetObjectDetailedMeta(req.Filename)
	if err != nil {
		logx.Errorf("ossGetObjectMetaError:%v", err)
		// 如果获取元数据失败，继续下载，但使用默认的Content-Type
	}

	// 设置响应头
	ext := strings.ToLower(filepath.Ext(req.Filename))
	contentType := getContentType(ext)

	if props != nil {
		// 如果获取到了元数据，使用OSS中存储的Content-Type
		if ct := props.Get("Content-Type"); ct != "" {
			contentType = ct
		}
		// 设置Content-Length
		if cl := props.Get("Content-Length"); cl != "" {
			l.responseWriter.Header().Set("Content-Length", cl)
		}
	}

	l.responseWriter.Header().Set("Content-Type", contentType)
	l.responseWriter.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", req.Filename))
	l.responseWriter.Header().Set("Cache-Control", "public, max-age=31536000") // 缓存1年

	// 将文件内容流式传输给客户端
	_, err = io.Copy(l.responseWriter, reader)
	if err != nil {
		logx.Errorf("copyFileToResponseError:%v", err)
		return nil
	}

	logx.Infof("文件下载成功: %s", req.Filename)
	return nil
}

// getContentType 根据文件扩展名返回Content-Type
func getContentType(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	default:
		return "application/octet-stream"
	}
}
