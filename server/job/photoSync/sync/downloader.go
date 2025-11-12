package sync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
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
	baseTimeout    time.Duration
	maxTimeout     time.Duration
	maxRetries     int
	retryBaseDelay time.Duration
}

// NewPhotoDownloader 创建照片下载器
func NewPhotoDownloader(baseTimeoutSeconds int, maxTimeoutSeconds int, maxRetries int, retryBaseDelaySeconds int) *PhotoDownloader {
	return &PhotoDownloader{
		baseTimeout:    time.Duration(baseTimeoutSeconds) * time.Second,
		maxTimeout:     time.Duration(maxTimeoutSeconds) * time.Second,
		maxRetries:     maxRetries,
		retryBaseDelay: time.Duration(retryBaseDelaySeconds) * time.Second,
	}
}

// DownloadPhoto 下载照片（带超时自适应重试，仅在失败时输出错误日志）
func (pd *PhotoDownloader) DownloadPhoto(ctx context.Context, photoUrl, destPath string) error {
	// 解析并校验 URL
	parsedURL, err := url.Parse(photoUrl)
	if err != nil {
		logx.Errorf("下载失败 url=%s, 原因=解析URL失败: %v", photoUrl, err)
		return fmt.Errorf("解析URL失败: %v", err)
	}
	if !parsedURL.IsAbs() {
		logx.Errorf("下载失败 url=%s, 原因=URL不是绝对URL", photoUrl)
		return fmt.Errorf("URL不是绝对URL: %s", photoUrl)
	}

	// http.Client 不设全局超时，使用每次请求的 context 控制
	client := &http.Client{}

	var lastErr error
	attempts := pd.maxRetries + 1
	for attempt := 0; attempt < attempts; attempt++ {
		// 计算本次尝试的超时时间：base * 2^attempt，上限为 maxTimeout
		attemptTimeout := pd.baseTimeout * time.Duration(1<<attempt)
		if attemptTimeout > pd.maxTimeout {
			attemptTimeout = pd.maxTimeout
		}

		// 使用带超时的子上下文，避免无限期阻塞大文件下载
		reqCtx, cancel := context.WithTimeout(ctx, attemptTimeout)

		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, photoUrl, nil)
		if err != nil {
			lastErr = fmt.Errorf("创建HTTP请求失败: %v", err)
			logx.Errorf("下载失败 url=%s, 原因=%v", photoUrl, lastErr)
			// 创建请求失败无需重试过多
			break
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			// 判断是否为超时/上下文截止，若可重试则退避后继续
			if isTimeoutLikeError(err) && attempt < attempts-1 {
				backoff := pd.retryBaseDelay * time.Duration(1<<attempt)
				if backoff > 0 {
					time.Sleep(backoff)
				}
				cancel()
				continue
			}
			logx.Errorf("下载失败 url=%s, 原因=%v", photoUrl, err)
			cancel()
			break
		}

		// 确保响应体关闭
		func() {
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				lastErr = fmt.Errorf("HTTP状态码=%d", resp.StatusCode)
				logx.Errorf("下载失败 url=%s, 原因=%v", photoUrl, lastErr)
				return
			}

			// 确保目标目录存在
			destDir := filepath.Dir(destPath)
			if err := os.MkdirAll(destDir, 0755); err != nil {
				lastErr = fmt.Errorf("创建目标目录失败: %v", err)
				logx.Errorf("下载失败 url=%s, 原因=%v", photoUrl, lastErr)
				return
			}

			// 清理可能存在的旧临时文件，避免残留数据导致问题
			os.Remove(destPath)

			out, err := os.Create(destPath)
			if err != nil {
				lastErr = fmt.Errorf("创建临时文件失败: %v", err)
				logx.Errorf("下载失败 url=%s, 原因=%v", photoUrl, lastErr)
				return
			}

			// 获取期望的文件大小（如果服务器提供了 Content-Length）
			expectedSize := resp.ContentLength

			// 复制响应体到文件
			written, err := io.Copy(out, resp.Body)
			if err != nil {
				out.Close()
				os.Remove(destPath)
				lastErr = fmt.Errorf("保存文件内容失败: %v", err)
				// 若为可重试超时类错误，尝试重试
				if isTimeoutLikeError(err) && attempt < attempts-1 {
					backoff := pd.retryBaseDelay * time.Duration(1<<attempt)
					if backoff > 0 {
						time.Sleep(backoff)
					}
					return
				}
				logx.Errorf("下载失败 url=%s, 原因=%v", photoUrl, lastErr)
				return
			}

			// 验证文件大小是否与预期一致（如果服务器提供了 Content-Length）
			if expectedSize > 0 && written != expectedSize {
				out.Close()
				os.Remove(destPath)
				lastErr = fmt.Errorf("文件大小不匹配: 期望 %d 字节, 实际写入 %d 字节", expectedSize, written)
				logx.Errorf("下载失败 url=%s, 原因=%v", photoUrl, lastErr)
				return
			}

			// 强制将缓冲区数据刷新到磁盘，确保数据完整性（关键步骤！）
			if err := out.Sync(); err != nil {
				out.Close()
				os.Remove(destPath)
				lastErr = fmt.Errorf("同步文件到磁盘失败: %v", err)
				logx.Errorf("下载失败 url=%s, 原因=%v", photoUrl, lastErr)
				return
			}

			// 关闭文件
			if err := out.Close(); err != nil {
				os.Remove(destPath)
				lastErr = fmt.Errorf("关闭文件失败: %v", err)
				logx.Errorf("下载失败 url=%s, 原因=%v", photoUrl, lastErr)
				return
			}

			// 验证临时文件确实存在且大小正确
			stat, err := os.Stat(destPath)
			if err != nil {
				os.Remove(destPath)
				lastErr = fmt.Errorf("验证临时文件失败: %v", err)
				logx.Errorf("下载失败 url=%s, 原因=%v", photoUrl, lastErr)
				return
			}
			if stat.Size() != written {
				os.Remove(destPath)
				lastErr = fmt.Errorf("文件大小验证失败: 写入 %d 字节, 实际 %d 字节", written, stat.Size())
				logx.Errorf("下载失败 url=%s, 原因=%v", photoUrl, lastErr)
				return
			}

			// 记录成功日志，包含文件大小信息
			logx.Infof("文件下载成功: url=%s, 大小=%.2fMB", photoUrl, float64(written)/(1024*1024))

			// 成功则清空 lastErr
			lastErr = nil
		}()

		// 结束本轮超时上下文
		cancel()

		if lastErr == nil {
			return nil
		}

		// 若可重试，执行退避
		if attempt < attempts-1 {
			if isTimeoutLikeError(lastErr) {
				backoff := pd.retryBaseDelay * time.Duration(1<<attempt)
				if backoff > 0 {
					time.Sleep(backoff)
				}
				continue
			}
			// 对非超时错误也仅重试有限次
			backoff := pd.retryBaseDelay * time.Duration(1<<attempt)
			if backoff > 0 {
				time.Sleep(backoff)
			}
			continue
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("未知错误")
	}
	return lastErr
}

// isTimeoutLikeError 判断错误是否为超时/上下文截止
func isTimeoutLikeError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// net.Error 接口的 Timeout
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return true
	}
	// 常见字符串匹配兜底
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "context deadline exceeded")
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
