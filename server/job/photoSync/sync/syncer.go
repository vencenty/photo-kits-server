package sync

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"server/job/photoSync/config"
	"server/model"
)

// PhotoSyncer 照片同步器
type PhotoSyncer struct {
	config        config.SyncConfig
	db            sqlx.SqlConn
	photoModel    model.PhotoModel
	orderModel    model.OrderModel
	imageAnalyzer *ImageAnalyzer
	downloader    *PhotoDownloader
	fileManager   *FileManager
}

// NewPhotoSyncer 创建一个新的照片同步器
func NewPhotoSyncer(db sqlx.SqlConn, syncConfig config.SyncConfig) *PhotoSyncer {
	return &PhotoSyncer{
		config:        syncConfig,
		db:            db,
		photoModel:    model.NewPhotoModel(db),
		orderModel:    model.NewOrderModel(db),
		imageAnalyzer: NewImageAnalyzer(),
		downloader:    NewPhotoDownloader(syncConfig.DownloadTimeout),
		fileManager:   NewFileManager(syncConfig.OutputPath),
	}
}

// SyncPhotos 执行照片同步操作，获取一条status=0或status=-1的订单并下载照片
func (s *PhotoSyncer) SyncPhotos(ctx context.Context) error {
	logx.Info("开始同步照片...")
	logx.Infof("输出根目录：%s", s.config.OutputPath)

	// 检查输出目录是否存在，不存在则创建
	if err := s.fileManager.EnsureOutputDirectory(); err != nil {
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

	// 按规格和比例分组处理照片
	successCount, failCount = s.downloadAllPhotos(ctx, photos, order)

	logx.Infof("订单 %s (ID: %d) 处理完成, 成功: %d, 失败: %d",
		order.OrderSn, order.Id, successCount, failCount)

	return successCount, failCount
}

// downloadAllPhotos 下载订单的所有照片，按规格和比例分类存储
func (s *PhotoSyncer) downloadAllPhotos(ctx context.Context, photos []*model.Photo, order *model.Order) (successCount, failCount int) {
	// 记录每个目录下的文件计数器，用于生成简单的数字文件名
	dirCounters := make(map[string]int)

	for i, photo := range photos {
		spec := photo.Spec
		if spec == "" {
			spec = "默认规格"
		}

		logx.Infof("处理照片 %d/%d: ID: %d, 规格: %s, URL: %s",
			i+1, len(photos), photo.Id, spec, photo.OriginUrl)

		// 下载照片到临时位置以获取尺寸信息
		tempFileName := fmt.Sprintf("temp_%d_%s", photo.Id, s.downloader.GetCleanFileName(photo.OriginUrl))
		tempDir := filepath.Join(s.config.OutputPath, "temp")

		// 确保临时目录存在
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			logx.Errorf("创建临时目录失败: %v", err)
			s.updatePhotoStatus(ctx, photo.Id, model.PhotoStatusFailed, err.Error())
			failCount++
			continue
		}
		tempPath := filepath.Join(tempDir, tempFileName)

		logx.Infof("开始下载照片到临时位置: ID: %d, 临时路径: %s", photo.Id, tempPath)

		if err := s.downloader.DownloadPhoto(ctx, photo.OriginUrl, tempPath); err != nil {
			logx.Errorf("照片下载失败, 照片ID: %d, URL: %s, 错误: %v", photo.Id, photo.OriginUrl, err)
			s.updatePhotoStatus(ctx, photo.Id, model.PhotoStatusFailed, err.Error())
			failCount++
			continue
		}

		// 获取图片尺寸并分析比例
		width, height, err := s.imageAnalyzer.GetImageDimensions(tempPath)
		if err != nil {
			logx.Errorf("获取图片尺寸失败, 照片ID: %d, 错误: %v", photo.Id, err)
			// 删除临时文件
			os.Remove(tempPath)
			s.updatePhotoStatus(ctx, photo.Id, model.PhotoStatusFailed, err.Error())
			failCount++
			continue
		}

		// 计算宽高比并确定分类
		ratio := s.imageAnalyzer.CalculateAspectRatio(width, height)
		aspectCategory := s.imageAnalyzer.GetAspectRatioCategory(ratio)

		logx.Infof("照片尺寸: %dx%d, 宽高比: %.3f, 分类: %s", width, height, ratio, aspectCategory)

		// 创建基于规格的目录结构
		finalDir, err := s.fileManager.CreateSpecBasedDirectories(order, spec, aspectCategory)
		if err != nil {
			logx.Errorf("创建目录失败: %v", err)
			// 删除临时文件
			os.Remove(tempPath)
			s.updatePhotoStatus(ctx, photo.Id, model.PhotoStatusFailed, err.Error())
			failCount++
			continue
		}

		// 为该目录生成下一个文件序号
		dirCounters[finalDir]++
		fileCounter := dirCounters[finalDir]

		// 提取原始文件扩展名
		originalExt := s.getFileExtension(photo.OriginUrl)
		if originalExt == "" {
			originalExt = ".jpg" // 默认扩展名
		}

		// 生成简单的数字文件名
		fileName := fmt.Sprintf("%d%s", fileCounter, originalExt)
		finalPath := filepath.Join(finalDir, fileName)

		// 移动临时文件到最终位置
		if err := s.moveFile(tempPath, finalPath); err != nil {
			logx.Errorf("移动文件到最终位置失败: %v", err)
			// 删除临时文件
			os.Remove(tempPath)
			s.updatePhotoStatus(ctx, photo.Id, model.PhotoStatusFailed, err.Error())
			failCount++
			continue
		}

		logx.Infof("照片下载成功: ID: %d, 保存路径: %s, 文件名: %s, 尺寸: %dx%d, 比例: %s",
			photo.Id, finalPath, fileName, width, height, aspectCategory)
		s.updatePhotoStatus(ctx, photo.Id, model.PhotoStatusSuccess, "")
		successCount++
	}

	return successCount, failCount
}

// getFileExtension 从URL中提取文件扩展名
func (s *PhotoSyncer) getFileExtension(url string) string {
	// 先去掉URL参数
	if idx := strings.Index(url, "?"); idx != -1 {
		url = url[:idx]
	}

	// 提取扩展名
	ext := filepath.Ext(url)
	if ext != "" {
		return strings.ToLower(ext)
	}

	return ""
}

// updatePhotoStatus 更新照片状态
func (s *PhotoSyncer) updatePhotoStatus(ctx context.Context, photoId uint64, status int64, errMsg string) {
	if err := s.photoModel.UpdateStatus(ctx, photoId, status, errMsg); err != nil {
		logx.Errorf("更新照片状态失败, 照片ID: %d, 错误: %v", photoId, err)
	}
}

// moveFile 移动文件
func (s *PhotoSyncer) moveFile(src, dst string) error {
	// 首先尝试直接重命名（在同一文件系统上这是原子操作）
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// 如果重命名失败，则复制文件然后删除源文件
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %v", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %v", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		os.Remove(dst) // 清理部分写入的文件
		return fmt.Errorf("复制文件内容失败: %v", err)
	}

	// 删除源文件
	if err := os.Remove(src); err != nil {
		logx.Errorf("删除临时文件失败: %v", err)
		// 不返回错误，因为文件已经成功复制
	}

	return nil
}
