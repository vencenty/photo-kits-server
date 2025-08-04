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
	logx.Infof("订单信息: ID: %d, 订单号: %s, 收货人: %s, 重试次数: %d", order.Id, order.OrderSn, order.Receiver, order.RetryCount)

	// 清理订单之前的旧文件
	if err := s.fileManager.CleanOrderFiles(order); err != nil {
		logx.Errorf("清理订单旧文件失败, 订单ID: %d, 错误: %v", order.Id, err)
		// 不因为清理失败而中断处理，继续执行
	}

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
	if failCount == 0 {
		// 全部成功 - status = 2 (已完成)
		logx.Infof("订单 %s 处理结果: 成功 %d 张, 失败 %d 张, 状态更新为: 同步成功",
			orderSn, successCount, failCount)
		return s.orderModel.UpdateStatus(ctx, orderId, model.OrderStatusCompleted)
	} else {
		// 有失败 - 增加重试次数，状态设为失败（可能重试）
		errorMsg := fmt.Sprintf("订单处理失败: 成功 %d 张, 失败 %d 张", successCount, failCount)
		logx.Infof("订单 %s 处理结果: 成功 %d 张, 失败 %d 张, 增加重试次数",
			orderSn, successCount, failCount)
		return s.orderModel.UpdateFailureCount(ctx, orderId, errorMsg)
	}
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
	// 记录已使用的文件名，避免重复
	usedFileNames := make(map[string]bool)
	// 照片序号计数器，从1开始
	photoIndex := 0

	for i, photo := range photos {
		photoIndex++ // 为每张照片分配一个序号
		spec := photo.Spec
		if spec == "" {
			spec = "默认规格"
		}

		// 获取该照片需要下载的数量，默认为1
		downloadCount := photo.Num
		if downloadCount <= 0 {
			downloadCount = 1
		}

		logx.Infof("处理照片 %d/%d: ID: %d, 规格: %s, 需要下载数量: %d, URL: %s",
			i+1, len(photos), photo.Id, spec, downloadCount, photo.OriginUrl)

		// 记录该照片的成功和失败次数
		photoSuccessCount := 0
		photoFailCount := 0

		// 保存第一次分析的结果，供后续副本使用
		var firstAnalysisResult struct {
			width          int
			height         int
			ratio          float64
			aspectCategory string
			finalDir       string
		}

		// 根据num字段循环下载指定数量的文件
		for copyIndex := 1; copyIndex <= int(downloadCount); copyIndex++ {
			logx.Infof("下载照片副本 %d/%d: 照片ID: %d", copyIndex, downloadCount, photo.Id)

			// 下载照片到临时位置以获取尺寸信息
			tempFileName := fmt.Sprintf("temp_%d_%d_%s", photo.Id, copyIndex, s.downloader.GetCleanFileName(photo.OriginUrl))
			tempDir := filepath.Join(s.config.OutputPath, "temp")

			// 确保临时目录存在
			if err := os.MkdirAll(tempDir, 0755); err != nil {
				logx.Errorf("创建临时目录失败: %v", err)
				photoFailCount++
				continue
			}
			tempPath := filepath.Join(tempDir, tempFileName)

			logx.Infof("开始下载照片到临时位置: ID: %d, 副本: %d, 临时路径: %s", photo.Id, copyIndex, tempPath)

			if err := s.downloader.DownloadPhoto(ctx, photo.OriginUrl, tempPath); err != nil {
				logx.Errorf("照片下载失败, 照片ID: %d, 副本: %d, URL: %s, 错误: %v", photo.Id, copyIndex, photo.OriginUrl, err)
				photoFailCount++
				continue
			}

			var finalDir string

			if copyIndex == 1 {
				// 第一次下载，需要分析图片
				width, height, err := s.imageAnalyzer.GetImageDimensions(tempPath)
				if err != nil {
					// 这种情况现在应该很少发生，因为image_analyzer会返回默认尺寸
					logx.Errorf("获取图片尺寸失败, 照片ID: %d, 错误: %v", photo.Id, err)
					// 删除临时文件
					os.Remove(tempPath)
					photoFailCount++
					continue
				}

				// 计算宽高比并确定分类
				ratio := s.imageAnalyzer.CalculateAspectRatio(width, height)
				aspectCategory := s.imageAnalyzer.GetAspectRatioCategory(ratio)

				// 检查是否使用了默认尺寸
				if width == 4000 && height == 3000 {
					logx.Infof("照片尺寸: %dx%d (使用默认尺寸), 宽高比: %.3f, 分类: %s", width, height, ratio, aspectCategory)
				} else {
					logx.Infof("照片尺寸: %dx%d, 宽高比: %.3f, 分类: %s", width, height, ratio, aspectCategory)
				}

				// 创建基于规格的目录结构
				finalDir, err = s.fileManager.CreateSpecBasedDirectories(order, spec, aspectCategory)
				if err != nil {
					logx.Errorf("创建目录失败: %v", err)
					// 删除临时文件
					os.Remove(tempPath)
					photoFailCount++
					continue
				}

				// 保存分析结果供后续副本使用
				firstAnalysisResult.width = width
				firstAnalysisResult.height = height
				firstAnalysisResult.ratio = ratio
				firstAnalysisResult.aspectCategory = aspectCategory
				firstAnalysisResult.finalDir = finalDir
			} else {
				// 后续副本，复用第一次的分析结果
				finalDir = firstAnalysisResult.finalDir
			}

			// 提取原始文件扩展名
			originalExt := s.getFileExtension(photo.OriginUrl)
			if originalExt == "" {
				originalExt = ".jpg" // 默认扩展名
			}

			// 根据副本编号生成简化的数字文件名
			var fileName string
			if copyIndex == 1 {
				// 第一张：{序号}.jpg
				fileName = fmt.Sprintf("%d%s", photoIndex, originalExt)
			} else {
				// 副本：{序号}_copy{副本编号-1}.jpg (副本从copy1开始)
				fileName = fmt.Sprintf("%d_copy%d%s", photoIndex, copyIndex-1, originalExt)
			}

			// 确保文件名唯一性，避免同一目录下的重复文件名
			finalPath := filepath.Join(finalDir, fileName)
			uniqueKey := fmt.Sprintf("%s/%s", finalDir, fileName)
			counter := 1
			for usedFileNames[uniqueKey] {
				if copyIndex == 1 {
					fileName = fmt.Sprintf("%d_%d%s", photoIndex, counter, originalExt)
				} else {
					fileName = fmt.Sprintf("%d_copy%d_%d%s", photoIndex, copyIndex-1, counter, originalExt)
				}
				finalPath = filepath.Join(finalDir, fileName)
				uniqueKey = fmt.Sprintf("%s/%s", finalDir, fileName)
				counter++
			}
			usedFileNames[uniqueKey] = true

			// 移动临时文件到最终位置
			if err := s.moveFile(tempPath, finalPath); err != nil {
				logx.Errorf("移动文件到最终位置失败: %v", err)
				// 删除临时文件
				os.Remove(tempPath)
				photoFailCount++
				continue
			}

			logx.Infof("照片副本下载成功: ID: %d, 序号: %d, 副本: %d/%d, 文件名: %s, 保存路径: %s",
				photo.Id, photoIndex, copyIndex, downloadCount, fileName, finalPath)
			photoSuccessCount++
		}

		// 更新照片状态
		if photoFailCount == 0 {
			// 所有副本都下载成功
			s.updatePhotoStatus(ctx, photo.Id, model.PhotoStatusSuccess, "")
			logx.Infof("照片所有副本下载完成: ID: %d, 成功: %d/%d", photo.Id, photoSuccessCount, downloadCount)
		} else if photoSuccessCount > 0 {
			// 部分成功
			errorMsg := fmt.Sprintf("部分下载成功: 成功 %d 个，失败 %d 个", photoSuccessCount, photoFailCount)
			s.updatePhotoStatus(ctx, photo.Id, model.PhotoStatusFailed, errorMsg)
			logx.Errorf("照片部分下载失败: ID: %d, %s", photo.Id, errorMsg)
		} else {
			// 全部失败
			errorMsg := fmt.Sprintf("所有副本下载失败: 失败 %d 个", photoFailCount)
			s.updatePhotoStatus(ctx, photo.Id, model.PhotoStatusFailed, errorMsg)
			logx.Errorf("照片全部下载失败: ID: %d, %s", photo.Id, errorMsg)
		}

		// 累计总体统计
		successCount += photoSuccessCount
		failCount += photoFailCount
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

// getFileNameWithoutExt 从URL中提取文件名（不含扩展名）
func (s *PhotoSyncer) getFileNameWithoutExt(url string) string {
	// 先去掉URL参数
	if idx := strings.Index(url, "?"); idx != -1 {
		url = url[:idx]
	}

	// 提取文件名
	fileName := filepath.Base(url)
	if fileName == "" || fileName == "." || fileName == "/" {
		return ""
	}

	// 去掉扩展名
	ext := filepath.Ext(fileName)
	if ext != "" {
		fileName = fileName[:len(fileName)-len(ext)]
	}

	// 清理文件名，移除不安全的字符
	fileName = strings.ReplaceAll(fileName, " ", "_")
	fileName = strings.ReplaceAll(fileName, "(", "_")
	fileName = strings.ReplaceAll(fileName, ")", "_")
	fileName = strings.ReplaceAll(fileName, "[", "_")
	fileName = strings.ReplaceAll(fileName, "]", "_")
	fileName = strings.ReplaceAll(fileName, "{", "_")
	fileName = strings.ReplaceAll(fileName, "}", "_")
	fileName = strings.ReplaceAll(fileName, "#", "_")
	fileName = strings.ReplaceAll(fileName, "%", "_")
	fileName = strings.ReplaceAll(fileName, "&", "_")
	fileName = strings.ReplaceAll(fileName, "?", "_")
	fileName = strings.ReplaceAll(fileName, "!", "_")

	return fileName
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
