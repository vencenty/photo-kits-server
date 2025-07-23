package sync

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// PhotoDownloader 照片下载器
type PhotoDownloader struct {
	downloadTimeout time.Duration
}

// NewPhotoDownloader 创建照片下载器
func NewPhotoDownloader(timeoutSeconds int) *PhotoDownloader {
	return &PhotoDownloader{
		downloadTimeout: time.Duration(timeoutSeconds) * time.Second,
	}
}

// DownloadPhoto 下载照片
func (pd *PhotoDownloader) DownloadPhoto(ctx context.Context, photoUrl, destPath string) error {
	logx.Infof("开始下载照片: URL=%s, 目标路径=%s, 超时时间=%v", photoUrl, destPath, pd.downloadTimeout)

	// 解析URL
	parsedURL, err := url.Parse(photoUrl)
	if err != nil {
		logx.Errorf("解析URL失败: %s, 错误: %v", photoUrl, err)
		return fmt.Errorf("解析URL失败: %v", err)
	}

	// 确保URL是绝对URL
	if !parsedURL.IsAbs() {
		logx.Errorf("URL不是绝对URL: %s", photoUrl)
		return fmt.Errorf("URL不是绝对URL: %s", photoUrl)
	}

	// 创建HTTP客户端，设置超时
	client := &http.Client{
		Timeout: pd.downloadTimeout,
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, photoUrl, nil)
	if err != nil {
		logx.Errorf("创建HTTP请求失败: %s, 错误: %v", photoUrl, err)
		return fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 发送请求
	logx.Infof("发送HTTP请求: %s", photoUrl)
	resp, err := client.Do(req)
	if err != nil {
		logx.Errorf("HTTP请求失败: %s, 错误: %v", photoUrl, err)
		return fmt.Errorf("HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	logx.Infof("HTTP响应状态: %d, URL: %s", resp.StatusCode, photoUrl)
	if resp.StatusCode != http.StatusOK {
		logx.Errorf("HTTP响应状态码不是200: %d, URL: %s", resp.StatusCode, photoUrl)
		return fmt.Errorf("HTTP响应状态码不是200: %d", resp.StatusCode)
	}

	// 确保目标目录存在
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		logx.Errorf("创建目标目录失败: %s, 错误: %v", destDir, err)
		return fmt.Errorf("创建目标目录失败: %v", err)
	}

	// 创建临时文件
	tempPath := destPath + ".tmp"
	logx.Infof("创建临时文件: %s", tempPath)
	out, err := os.Create(tempPath)
	if err != nil {
		logx.Errorf("创建临时文件失败: %s, 错误: %v", tempPath, err)
		return fmt.Errorf("创建临时文件失败: %v", err)
	}
	defer out.Close()

	// 复制内容
	logx.Infof("开始复制文件内容: %s", tempPath)
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		os.Remove(tempPath)
		logx.Errorf("保存文件内容失败: %s, 错误: %v", tempPath, err)
		return fmt.Errorf("保存文件内容失败: %v", err)
	}

	// 关闭文件
	out.Close()

	// 原子性重命名，避免部分写入的文件
	logx.Infof("重命名文件: %s -> %s", tempPath, destPath)
	if err := os.Rename(tempPath, destPath); err != nil {
		os.Remove(tempPath)
		logx.Errorf("重命名文件失败: %s -> %s, 错误: %v", tempPath, destPath, err)
		return fmt.Errorf("重命名文件失败: %v", err)
	}

	logx.Infof("照片下载成功: %s", destPath)
	return nil
}

// GetCleanFileName 从URL中提取不含查询参数的文件名
func (pd *PhotoDownloader) GetCleanFileName(fileUrl string) string {
	// 先获取URL的基本文件名
	fileName := filepath.Base(fileUrl)

	// 移除查询参数部分
	if queryIndex := strings.Index(fileName, "?"); queryIndex > 0 {
		fileName = fileName[:queryIndex]
	}

	return fileName
}
