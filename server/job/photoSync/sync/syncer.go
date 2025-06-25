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
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"photo-kits-server/server/job/photoSync/config"
	"photo-kits-server/server/model"
)

// PhotoSyncer 照片同步器
type PhotoSyncer struct {
	config     config.SyncConfig
	db         sqlx.SqlConn
	photoModel model.PhotoModel
	orderModel model.OrderModel
}

// NewPhotoSyncer 创建一个新的照片同步器
func NewPhotoSyncer(db sqlx.SqlConn, syncConfig config.SyncConfig) *PhotoSyncer {
	return &PhotoSyncer{
		config:     syncConfig,
		db:         db,
		photoModel: model.NewPhotoModel(db),
		orderModel: model.NewOrderModel(db),
	}
}

// SyncPhotos 执行照片同步操作，获取一条status=0或status=-1的订单并下载照片
func (s *PhotoSyncer) SyncPhotos(ctx context.Context) error {
	logx.Info("开始同步照片...")
	logx.Infof("输出根目录：%s", s.config.OutputPath)

	// 检查输出目录是否存在，不存在则创建
	if err := os.MkdirAll(s.config.OutputPath, 0755); err != nil {
		logx.Errorf("无法创建输出目录: %v", err)
		return err
	}

	// 获取一条待处理的订单(status=0或status=-1)并立即标记为处理中(status=1)
	order, err := s.orderModel.GetAndLockPendingOrder(ctx)
	if err != nil {
		if err.Error() == "record not found" || err.Error() == "sql: no rows in result set" {
			logx.Info("没有找到待处理的订单")
			return nil
		}
		logx.Errorf("获取待处理订单失败: %v", err)
		return err
	}

	if order == nil {
		logx.Info("没有找到待处理的订单")
		return nil
	}

	logx.Infof("===== 开始处理订单 =====")
	logx.Infof("订单信息: ID: %d, 订单号: %s, 收货人: %s", order.Id, order.OrderSn, order.Receiver)

	// 处理订单照片
	successCount, failCount := s.processOrderPhotos(ctx, order)

	// 根据结果更新订单状态
	if err := s.updateOrderStatusByResult(ctx, order.Id, order.OrderSn, successCount, failCount); err != nil {
		logx.Errorf("更新订单最终状态失败, 订单ID: %d, 错误: %v", order.Id, err)
	}

	logx.Infof("订单处理完成, 订单ID: %d, 订单号: %s", order.Id, order.OrderSn)
	logx.Infof("===== 订单处理结束 =====")

	logx.Info("照片同步完成")
	return nil
}

// updateOrderStatusByResult 根据下载结果更新订单状态
func (s *PhotoSyncer) updateOrderStatusByResult(ctx context.Context, orderId uint64, orderSn string, successCount, failCount int) error {
	var newStatus int64
	var statusName string

	if failCount == 0 {
		// 全部成功 - status = 2 (已完成)
		newStatus = model.OrderStatusCompleted
		statusName = "同步成功"
	} else {
		// 有失败 - status = -1 (失败，下次会重新处理)
		newStatus = model.OrderStatusFailed
		statusName = "同步失败"
	}

	logx.Infof("订单 %s 处理结果: 成功 %d 张, 失败 %d 张, 状态更新为: %s",
		orderSn, successCount, failCount, statusName)

	return s.orderModel.UpdateStatus(ctx, orderId, newStatus)
}

// processOrderPhotos 处理订单的照片
func (s *PhotoSyncer) processOrderPhotos(ctx context.Context, order *model.Order) (successCount, failCount int) {
	// 查询订单的所有照片
	photos, err := s.photoModel.FindByOrderId(ctx, order.Id)
	if err != nil {
		logx.Errorf("查询订单照片失败: %v", err)
		return 0, 0
	}

	if len(photos) == 0 {
		logx.Infof("订单 %d 没有照片需要处理", order.Id)
		return 0, 0
	}

	logx.Infof("订单 %d 有 %d 张照片需要处理", order.Id, len(photos))

	// 使用订单创建时间创建目录
	baseTime := order.CreatedAt
	logx.Infof("订单 %s 创建时间: %s，将同步到对应日期目录", order.OrderSn, baseTime.Format("2006-01-02 15:04:05"))

	// 创建订单目录
	orderDir, err := s.createOrderDirectories(order, baseTime)
	if err != nil {
		logx.Errorf("创建订单目录失败: %v", err)
		return 0, len(photos) // 目录创建失败，所有照片都算失败
	}

	// 判断是否需要清空目录（订单原状态为0表示重新编辑过，需要清空）
	// 注意：此时订单状态已经被更新为1(处理中)，需要通过其他方式判断是否需要清空
	// 简化处理：总是清空目录，确保每次都是全新同步
	logx.Infof("订单 %s 清空目录重新同步", order.OrderSn)
	if err := s.cleanOrderDirectory(orderDir); err != nil {
		logx.Errorf("清空订单目录失败: %v", err)
		return 0, len(photos) // 清空失败，所有照片都算失败
	}
	logx.Infof("已清空订单目录: %s", orderDir)

	// 下载所有照片
	successCount, failCount = s.downloadAllPhotos(ctx, photos, orderDir)

	logx.Infof("订单 %s (ID: %d) 处理完成, 成功: %d, 失败: %d, 总目录: %s",
		order.OrderSn, order.Id, successCount, failCount, orderDir)

	return successCount, failCount
}

// downloadAllPhotos 下载订单的所有照片
func (s *PhotoSyncer) downloadAllPhotos(ctx context.Context, photos []*model.Photo, orderDir string) (successCount, failCount int) {
	// 按照照片的规格分组创建目录
	specDirs := make(map[string]string)
	// 记录每个规格目录下已使用的文件名
	usedFileNames := make(map[string]map[string]bool)

	for i, photo := range photos {
		// 使用规格作为目录名
		spec := photo.Spec
		if spec == "" {
			spec = "默认规格"
		}
		logx.Infof("处理照片 %d/%d: ID: %d, 规格: %s, URL: %s",
			i+1, len(photos), photo.Id, spec, photo.OriginUrl)

		// 确保规格目录存在
		specDir, err := s.ensureSpecDirectory(orderDir, spec, specDirs)
		if err != nil {
			logx.Errorf("创建规格目录失败: %v", err)
			s.updatePhotoStatus(ctx, photo.Id, model.PhotoStatusFailed, err.Error())
			failCount++
			continue
		}

		// 初始化该规格目录的文件名记录
		if usedFileNames[specDir] == nil {
			usedFileNames[specDir] = make(map[string]bool)
		}

		// 下载照片
		fileName := getCleanFileName(photo.OriginUrl)
		// 生成唯一的文件名，避免重复
		uniqueFileName := s.generateUniqueFileName(specDir, fileName, usedFileNames[specDir])
		destPath := filepath.Join(specDir, uniqueFileName)

		logx.Infof("开始下载照片: ID: %d, 原始文件名: %s, 实际文件名: %s, 目标目录: %s",
			photo.Id, fileName, uniqueFileName, specDir)

		if err := s.downloadPhoto(ctx, photo.OriginUrl, destPath); err != nil {
			logx.Errorf("照片下载失败, 照片ID: %d, URL: %s, 错误: %v", photo.Id, photo.OriginUrl, err)
			s.updatePhotoStatus(ctx, photo.Id, model.PhotoStatusFailed, err.Error())
			failCount++
			continue
		}

		// 标记文件名已使用
		usedFileNames[specDir][uniqueFileName] = true

		logx.Infof("照片下载成功: ID: %d, 保存路径: %s", photo.Id, destPath)
		s.updatePhotoStatus(ctx, photo.Id, model.PhotoStatusSuccess, "")
		successCount++
	}

	return successCount, failCount
}

// generateUniqueFileName 生成唯一的文件名，避免重复
func (s *PhotoSyncer) generateUniqueFileName(dir, fileName string, usedNames map[string]bool) string {
	// 检查原始文件名是否已使用
	if !usedNames[fileName] && !s.fileExists(filepath.Join(dir, fileName)) {
		return fileName
	}

	// 分离文件名和扩展名
	ext := filepath.Ext(fileName)
	nameWithoutExt := strings.TrimSuffix(fileName, ext)

	// 尝试添加数字后缀
	counter := 1
	for {
		newFileName := fmt.Sprintf("%s-%d%s", nameWithoutExt, counter, ext)

		// 检查是否在内存记录和文件系统中都不存在
		if !usedNames[newFileName] && !s.fileExists(filepath.Join(dir, newFileName)) {
			logx.Infof("生成唯一文件名: %s -> %s", fileName, newFileName)
			return newFileName
		}

		counter++

		// 防止无限循环
		if counter > 9999 {
			timestamp := time.Now().Format("150405")
			newFileName := fmt.Sprintf("%s-%s%s", nameWithoutExt, timestamp, ext)
			logx.Infof("达到最大重试次数，使用时间戳: %s -> %s", fileName, newFileName)
			return newFileName
		}
	}
}

// ensureSpecDirectory 确保规格目录存在，返回目录路径
func (s *PhotoSyncer) ensureSpecDirectory(orderDir, spec string, specDirs map[string]string) (string, error) {
	if specDir, exists := specDirs[spec]; exists {
		return specDir, nil
	}

	// 创建规格目录
	fullSpecDir := filepath.Join(orderDir, spec)
	logx.Infof("创建规格目录: %s", fullSpecDir)
	if err := os.MkdirAll(fullSpecDir, 0755); err != nil {
		return "", fmt.Errorf("创建规格目录失败 %s: %v", spec, err)
	}
	specDirs[spec] = fullSpecDir
	return fullSpecDir, nil
}

// updatePhotoStatus 更新照片状态
func (s *PhotoSyncer) updatePhotoStatus(ctx context.Context, photoId uint64, status int64, errMsg string) {
	if err := s.photoModel.UpdateStatus(ctx, photoId, status, errMsg); err != nil {
		logx.Errorf("更新照片状态失败, 照片ID: %d, 错误: %v", photoId, err)
	}
}

// createOrderDirectories 创建订单的目录结构，返回订单目录路径
func (s *PhotoSyncer) createOrderDirectories(order *model.Order, baseTime time.Time) (string, error) {
	// 创建年份目录（如："2025"）
	yearName := fmt.Sprintf("%d", baseTime.Year())
	yearDir := filepath.Join(s.config.OutputPath, yearName)
	if err := os.MkdirAll(yearDir, 0755); err != nil {
		return "", fmt.Errorf("创建年份目录失败: %v", err)
	}

	// 创建年月目录（如："202506"）
	monthName := fmt.Sprintf("%d%02d", baseTime.Year(), int(baseTime.Month()))
	monthDir := filepath.Join(yearDir, monthName)
	if err := os.MkdirAll(monthDir, 0755); err != nil {
		return "", fmt.Errorf("创建月份目录失败: %v", err)
	}

	// 创建年月日目录（如："20250601"）
	dateStr := baseTime.Format("20060102")
	dateDir := filepath.Join(monthDir, dateStr)
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		return "", fmt.Errorf("创建日期目录失败: %v", err)
	}

	// 创建以收货人姓名-订单号为名的目录
	orderDir := filepath.Join(dateDir, fmt.Sprintf("%s-%s", order.Receiver, order.OrderSn))
	if err := os.MkdirAll(orderDir, 0755); err != nil {
		return "", fmt.Errorf("创建订单目录失败: %v", err)
	}

	logx.Infof("为订单 %s 创建目录: %s (基于时间: %s)", order.OrderSn, orderDir, baseTime.Format("2006-01-02 15:04:05"))

	return orderDir, nil
}

// downloadPhoto 下载照片
func (s *PhotoSyncer) downloadPhoto(ctx context.Context, photoUrl, destPath string) error {
	timeout := time.Duration(s.config.DownloadTimeout) * time.Second
	logx.Infof("下载照片到: %s, 超时时间: %v", destPath, timeout)

	// 解析URL
	parsedURL, err := url.Parse(photoUrl)
	if err != nil {
		return fmt.Errorf("解析URL失败: %v", err)
	}

	// 确保URL是绝对URL
	if !parsedURL.IsAbs() {
		return fmt.Errorf("URL不是绝对URL: %s", photoUrl)
	}

	// 创建HTTP客户端，设置超时
	client := &http.Client{
		Timeout: timeout,
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, photoUrl, nil)
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP响应状态码不是200: %d", resp.StatusCode)
	}

	// 创建临时文件
	tempPath := destPath + ".tmp"
	out, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %v", err)
	}
	defer out.Close()

	// 复制内容
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("保存文件内容失败: %v", err)
	}

	// 关闭文件
	out.Close()

	// 原子性重命名，避免部分写入的文件
	if err := os.Rename(tempPath, destPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("重命名文件失败: %v", err)
	}

	return nil
}

// getCleanFileName 从URL中提取不含查询参数的文件名
func getCleanFileName(fileUrl string) string {
	// 先获取URL的基本文件名
	fileName := filepath.Base(fileUrl)

	// 移除查询参数部分
	if queryIndex := strings.Index(fileName, "?"); queryIndex > 0 {
		fileName = fileName[:queryIndex]
	}

	return fileName
}

// fileExists 检查文件是否存在
func (s *PhotoSyncer) fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

// cleanOrderDirectory 清空订单目录中的所有内容
func (s *PhotoSyncer) cleanOrderDirectory(orderDir string) error {
	// 检查目录是否存在
	if _, err := os.Stat(orderDir); os.IsNotExist(err) {
		logx.Infof("订单目录不存在，无需清空: %s", orderDir)
		return nil
	}

	logx.Infof("开始清空订单目录: %s", orderDir)

	// 打开订单目录
	dir, err := os.Open(orderDir)
	if err != nil {
		return fmt.Errorf("打开订单目录失败: %v", err)
	}
	defer dir.Close()

	// 读取目录中的所有文件和子目录
	files, err := dir.Readdir(0)
	if err != nil {
		return fmt.Errorf("读取目录内容失败: %v", err)
	}

	// 遍历所有文件和子目录，删除它们
	for _, file := range files {
		filePath := filepath.Join(orderDir, file.Name())

		if file.IsDir() {
			if err := os.RemoveAll(filePath); err != nil {
				logx.Errorf("删除目录失败: %s, 错误: %v", filePath, err)
				// 继续删除其他文件，不返回错误
			} else {
				logx.Infof("已删除目录: %s", filePath)
			}
		} else {
			if err := os.Remove(filePath); err != nil {
				logx.Errorf("删除文件失败: %s, 错误: %v", filePath, err)
				// 继续删除其他文件，不返回错误
			} else {
				logx.Infof("已删除文件: %s", filePath)
			}
		}
	}

	logx.Infof("订单目录清空完成: %s", orderDir)
	return nil
}
