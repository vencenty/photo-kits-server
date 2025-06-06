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

// RetryTask 重试任务
type RetryTask struct {
	PhotoID  uint64
	URL      string
	DestDir  string
	FileName string
	Attempt  int
	OrderID  uint64
}

// PhotoSyncer 照片同步器
type PhotoSyncer struct {
	config     config.SyncConfig
	db         sqlx.SqlConn
	photoModel model.PhotoModel
	orderModel model.OrderModel
	retryQueue chan *RetryTask
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewPhotoSyncer 创建一个新的照片同步器
func NewPhotoSyncer(db sqlx.SqlConn, syncConfig config.SyncConfig) *PhotoSyncer {
	ctx, cancel := context.WithCancel(context.Background())

	syncer := &PhotoSyncer{
		config:     syncConfig,
		db:         db,
		photoModel: model.NewPhotoModel(db),
		orderModel: model.NewOrderModel(db),
		retryQueue: make(chan *RetryTask, syncConfig.RetryChannelSize),
		ctx:        ctx,
		cancel:     cancel,
	}

	// 启动异步重试机制
	if syncConfig.AsyncRetry {
		syncer.startRetryWorkers()
	}

	return syncer
}

// startRetryWorkers 启动重试工作协程池
func (s *PhotoSyncer) startRetryWorkers() {
	for i := 0; i < s.config.RetryWorkers; i++ {
		go s.retryWorker(i)
	}
	logx.Infof("启动了 %d 个异步重试工作协程", s.config.RetryWorkers)
}

// Stop 停止同步器和所有重试协程
func (s *PhotoSyncer) Stop() {
	s.cancel()
	close(s.retryQueue)
}

// SyncPhotos 执行照片同步操作，扫描待处理订单并创建目录结构
func (s *PhotoSyncer) SyncPhotos(ctx context.Context) error {
	logx.Info("开始同步照片...")
	logx.Infof("输出根目录：%s", s.config.OutputPath)

	// 检查输出目录是否存在，不存在则创建
	if err := os.MkdirAll(s.config.OutputPath, 0755); err != nil {
		logx.Errorf("无法创建输出目录: %v", err)
		return err
	}

	// 处理失败照片和检查处理中订单状态
	if err := s.handleFailedPhotosAndProcessingOrders(ctx); err != nil {
		logx.Errorf("处理失败照片和检查订单状态时出错: %v", err)
		// 继续处理新订单，不中断整个过程
	}

	// 获取待处理订单
	batchSize := s.config.BatchSize
	pendingOrders, err := s.orderModel.FindPendingOrders(ctx, batchSize)
	if err != nil {
		logx.Errorf("查询待处理订单失败: %v", err)
		return err
	}

	if len(pendingOrders) == 0 {
		logx.Info("没有找到待处理的订单")
		return nil
	}

	logx.Infof("找到 %d 个待处理订单", len(pendingOrders))

	// 处理每个订单
	for _, order := range pendingOrders {
		logx.Infof("===== 开始处理订单 =====")
		logx.Infof("订单信息: ID: %d, 订单号: %s, 收货人: %s, 备注: %s",
			order.Id, order.OrderSn, order.Receiver, order.Remark)

		// 更新订单状态为处理中
		if err := s.orderModel.UpdateStatus(ctx, order.Id, model.OrderStatusProcessing); err != nil {
			logx.Errorf("更新订单状态失败, 订单ID: %d, 错误: %v", order.Id, err)
			continue
		}

		// 处理订单照片
		err = s.processOrderPhotos(ctx, order)
		if err != nil {
			logx.Errorf("处理订单照片失败, 订单ID: %d, 错误: %v", order.Id, err)
			// 判断错误类型：如果是照片下载失败，保持处理中状态，让重试机制处理
			// 如果是其他系统错误（如目录创建失败），将状态还原为待处理
			if strings.Contains(err.Error(), "张照片下载失败") {
				logx.Infof("订单 %d 部分照片下载失败，保持处理中状态，等待重试", order.Id)
				// 保持 Processing 状态，不更改
			} else {
				logx.Errorf("订单 %d 遇到系统错误，状态还原为待处理", order.Id)
				_ = s.orderModel.UpdateStatus(ctx, order.Id, model.OrderStatusPending)
			}
			continue
		}

		// 更新订单状态为已完成
		if err := s.orderModel.UpdateStatus(ctx, order.Id, model.OrderStatusCompleted); err != nil {
			logx.Errorf("更新订单状态失败, 订单ID: %d, 错误: %v", order.Id, err)
		} else {
			logx.Infof("订单处理完成, 订单ID: %d, 订单号: %s", order.Id, order.OrderSn)
		}
		logx.Infof("===== 订单处理结束 =====")
	}

	logx.Info("照片同步完成")
	return nil
}

// handleFailedPhotosAndProcessingOrders 统一处理失败照片重试和检查处理中订单状态
func (s *PhotoSyncer) handleFailedPhotosAndProcessingOrders(ctx context.Context) error {
	var totalSuccessCount, totalFailCount int

	// 1. 处理所有失败状态的照片
	logx.Info("开始处理失败状态的照片...")
	successCount, failCount, err := s.retryFailedPhotos(ctx)
	if err != nil {
		logx.Errorf("处理失败照片时出错: %v", err)
	} else {
		totalSuccessCount += successCount
		totalFailCount += failCount
		if successCount > 0 || failCount > 0 {
			logx.Infof("失败照片处理完成, 成功: %d, 失败: %d", successCount, failCount)
		}
	}

	// 2. 检查和处理处理中的订单
	logx.Info("开始检查处理中订单状态...")
	successCount, failCount, err = s.checkAndRetryProcessingOrders(ctx)
	if err != nil {
		logx.Errorf("检查处理中订单时出错: %v", err)
	} else {
		totalSuccessCount += successCount
		totalFailCount += failCount
		if successCount > 0 || failCount > 0 {
			logx.Infof("处理中订单检查完成, 成功: %d, 失败: %d", successCount, failCount)
		}
	}

	if totalSuccessCount > 0 || totalFailCount > 0 {
		logx.Infof("总计重试结果, 成功: %d, 失败: %d", totalSuccessCount, totalFailCount)
	}

	return nil
}

// retryFailedPhotos 重试所有失败状态的照片
func (s *PhotoSyncer) retryFailedPhotos(ctx context.Context) (successCount, failCount int, err error) {
	failedPhotos, err := s.photoModel.FindFailedPhotos(ctx, s.config.BatchSize)
	if err != nil {
		return 0, 0, fmt.Errorf("查询失败照片出错: %v", err)
	}

	if len(failedPhotos) == 0 {
		logx.Info("没有找到需要重试的失败照片")
		return 0, 0, nil
	}

	logx.Infof("找到 %d 张失败照片需要重试", len(failedPhotos))

	// 按订单ID分组
	photosByOrder := make(map[uint64][]*model.Photo)
	for _, photo := range failedPhotos {
		photosByOrder[photo.OrderId] = append(photosByOrder[photo.OrderId], photo)
	}

	// 处理每个订单的失败照片
	for orderId, photos := range photosByOrder {
		batchSuccessCount, batchFailCount, err := s.retryPhotosForOrder(ctx, orderId, photos, true)
		if err != nil {
			logx.Errorf("重试订单 %d 失败照片时出错: %v", orderId, err)
			continue
		}
		successCount += batchSuccessCount
		failCount += batchFailCount
	}

	return successCount, failCount, nil
}

// checkAndRetryProcessingOrders 检查处理中的订单并重试失败照片
func (s *PhotoSyncer) checkAndRetryProcessingOrders(ctx context.Context) (successCount, failCount int, err error) {
	processingOrders, err := s.orderModel.FindProcessingOrders(ctx, s.config.BatchSize)
	if err != nil {
		return 0, 0, fmt.Errorf("查询处理中订单出错: %v", err)
	}

	if len(processingOrders) == 0 {
		logx.Info("没有找到处理中的订单")
		return 0, 0, nil
	}

	logx.Infof("找到 %d 个处理中的订单", len(processingOrders))

	// 处理每个处理中的订单
	for _, order := range processingOrders {
		// 查询该订单的失败照片
		failedPhotos, err := s.photoModel.FindFailedPhotosByOrderId(ctx, order.Id)
		if err != nil {
			logx.Errorf("查询订单 %d 失败照片出错: %v", order.Id, err)
			continue
		}

		if len(failedPhotos) == 0 {
			// 没有失败照片，检查是否所有照片都已成功
			if s.checkAndCompleteOrder(ctx, order.Id) {
				logx.Infof("订单 %d 所有照片已完成，状态已更新为已完成", order.Id)
			}
			continue
		}

		logx.Infof("订单 %d 有 %d 张失败照片需要重试", order.Id, len(failedPhotos))

		// 重试失败照片（不清空目录，因为可能有成功的照片）
		batchSuccessCount, batchFailCount, err := s.retryPhotosForOrder(ctx, order.Id, failedPhotos, false)
		if err != nil {
			logx.Errorf("重试订单 %d 失败照片时出错: %v", order.Id, err)
			continue
		}

		successCount += batchSuccessCount
		failCount += batchFailCount

		logx.Infof("订单 %d 重试完成, 成功: %d, 失败: %d", order.Id, batchSuccessCount, batchFailCount)

		// 如果这次重试没有失败的照片，检查是否可以完成订单
		if batchFailCount == 0 {
			s.checkAndCompleteOrder(ctx, order.Id)
		}
	}

	return successCount, failCount, nil
}

// retryPhotosForOrder 为指定订单重试失败的照片
func (s *PhotoSyncer) retryPhotosForOrder(ctx context.Context, orderId uint64, photos []*model.Photo, cleanDirectory bool) (successCount, failCount int, err error) {
	// 查询订单信息
	order, err := s.orderModel.FindOne(ctx, orderId)
	if err != nil {
		return 0, 0, fmt.Errorf("查询订单信息失败, 订单ID: %d, 错误: %v", orderId, err)
	}

	// 创建目录结构
	orderDir, err := s.createOrderDirectories(order)
	if err != nil {
		return 0, 0, fmt.Errorf("为订单创建目录失败, 订单ID: %d, 错误: %v", orderId, err)
	}

	// 根据参数决定是否清空目录
	if cleanDirectory {
		if err := s.cleanOrderDirectory(orderDir); err != nil {
			logx.Errorf("清空订单目录失败: %v", err)
			return 0, 0, err
		}
		logx.Infof("已清空订单目录: %s", orderDir)
	}

	// 使用统一的照片处理方法
	return s.processPhotosInBatch(ctx, photos, orderDir, orderId)
}

// processPhotosInBatch 批量处理照片的通用方法
func (s *PhotoSyncer) processPhotosInBatch(ctx context.Context, photos []*model.Photo, orderDir string, orderID uint64) (successCount, failCount int, err error) {
	// 按照照片的规格分组
	specDirs := make(map[string]string)

	for i, photo := range photos {
		// 使用规格作为目录名
		spec := photo.Spec
		if spec == "" {
			spec = "默认规格" // 如果规格为空，使用默认规格
		}
		logx.Infof("处理照片 %d/%d: ID: %d, 规格: %s, URL: %s",
			i+1, len(photos), photo.Id, spec, photo.Url)

		// 检查该规格的目录是否已创建
		specDir, createErr := s.ensureSpecDirectory(orderDir, spec, specDirs)
		if createErr != nil {
			s.updatePhotoStatus(ctx, photo.Id, model.PhotoStatusFailed, createErr.Error())
			failCount++
			continue
		}

		// 下载照片到对应的目录
		fileName := getCleanFileName(photo.Url)
		logx.Infof("开始下载照片: ID: %d, 目标文件名: %s, 目标目录: %s",
			photo.Id, fileName, specDir)

		if downloadErr := s.downloadPhoto(ctx, photo.Url, specDir, fileName, photo.Id, orderID); downloadErr != nil {
			errMsg := fmt.Sprintf("下载照片失败: %v", downloadErr)

			// 区分超时错误和其他错误，提供更详细的日志
			if s.isTimeoutError(downloadErr) {
				logx.Errorf("照片下载超时, 照片ID: %d, URL: %s, 错误: %v", photo.Id, photo.Url, downloadErr)
			} else {
				logx.Errorf("照片下载失败, 照片ID: %d, URL: %s, 错误: %v", photo.Id, photo.Url, downloadErr)
			}

			s.updatePhotoStatus(ctx, photo.Id, model.PhotoStatusFailed, errMsg)
			failCount++
			continue
		}

		logx.Infof("照片下载成功: ID: %d, 保存路径: %s",
			photo.Id, filepath.Join(specDir, fileName))

		// 更新照片状态为成功
		s.updatePhotoStatus(ctx, photo.Id, model.PhotoStatusSuccess, "")
		successCount++
	}

	return successCount, failCount, nil
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

// updatePhotoStatus 更新照片状态的统一方法
func (s *PhotoSyncer) updatePhotoStatus(ctx context.Context, photoId uint64, status int64, errMsg string) {
	if err := s.photoModel.UpdateStatus(ctx, photoId, status, errMsg); err != nil {
		logx.Errorf("更新照片状态失败, 照片ID: %d, 错误: %v", photoId, err)
	}
}

// checkAndCompleteOrder 检查订单是否所有照片都已成功，如果是则更新为已完成状态
func (s *PhotoSyncer) checkAndCompleteOrder(ctx context.Context, orderId uint64) bool {
	allPhotos, err := s.photoModel.FindByOrderId(ctx, orderId)
	if err != nil {
		logx.Errorf("查询订单 %d 所有照片出错: %v", orderId, err)
		return false
	}

	for _, photo := range allPhotos {
		if photo.Status != model.PhotoStatusSuccess {
			return false
		}
	}

	logx.Infof("订单 %d 所有照片都已成功，更新状态为已完成", orderId)
	if err := s.orderModel.UpdateStatus(ctx, orderId, model.OrderStatusCompleted); err != nil {
		logx.Errorf("更新订单状态失败, 订单ID: %d, 错误: %v", orderId, err)
		return false
	}
	return true
}



// createOrderDirectories 创建订单的目录结构，返回订单目录路径
func (s *PhotoSyncer) createOrderDirectories(order *model.Order) (string, error) {
	today := time.Now()

	// 创建年份目录（如："2024"）
	yearName := fmt.Sprintf("%d", today.Year())
	yearDir := filepath.Join(s.config.OutputPath, yearName)
	if err := os.MkdirAll(yearDir, 0755); err != nil {
		return "", fmt.Errorf("创建年份目录失败: %v", err)
	}

	// 创建月份目录（如："6月"）
	monthName := fmt.Sprintf("%d月", int(today.Month()))
	monthDir := filepath.Join(yearDir, monthName)
	if err := os.MkdirAll(monthDir, 0755); err != nil {
		return "", fmt.Errorf("创建月份目录失败: %v", err)
	}

	// 创建以日期为名的目录（如："6-01"）
	dateStr := today.Format("1-02")
	dateDir := filepath.Join(monthDir, dateStr)
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		return "", fmt.Errorf("创建日期目录失败: %v", err)
	}

	// 创建以收货人姓名-订单号为名的目录
	orderDir := filepath.Join(dateDir, fmt.Sprintf("%s-%s", order.Receiver, order.OrderSn))
	if err := os.MkdirAll(orderDir, 0755); err != nil {
		return "", fmt.Errorf("创建订单目录失败: %v", err)
	}

	// 输出详细的目录路径
	logx.Infof("为订单 %s 创建目录: %s", order.OrderSn, orderDir)

	return orderDir, nil
}

// processOrderPhotos 处理订单的照片
func (s *PhotoSyncer) processOrderPhotos(ctx context.Context, order *model.Order) error {
	// 查询订单的所有照片
	photos, err := s.photoModel.FindByOrderId(ctx, order.Id)
	if err != nil {
		return fmt.Errorf("查询订单照片失败: %v", err)
	}

	if len(photos) == 0 {
		logx.Infof("订单 %d 没有照片需要处理", order.Id)
		return nil
	}

	logx.Infof("订单 %d 有 %d 张照片需要处理", order.Id, len(photos))

	// 创建订单目录
	orderDir, err := s.createOrderDirectories(order)
	if err != nil {
		return err
	}

	// 清空订单目录中的所有内容，确保每次同步都是最新的照片集合
	if err := s.cleanOrderDirectory(orderDir); err != nil {
		logx.Errorf("清空订单目录失败: %v", err)
		return err
	}
	logx.Infof("已清空订单目录: %s", orderDir)

	// 使用统一的照片处理方法
	successCount, failCount, err := s.processPhotosInBatch(ctx, photos, orderDir, order.Id)
	if err != nil {
		return err
	}

	logx.Infof("订单 %s (ID: %d) 处理完成, 成功: %d, 失败: %d, 总目录: %s",
		order.OrderSn, order.Id, successCount, failCount, orderDir)

	// 如果有照片下载失败，返回错误，防止订单被标记为已完成
	if failCount > 0 {
		return fmt.Errorf("订单 %s 有 %d 张照片下载失败，共 %d 张照片", order.OrderSn, failCount, len(photos))
	}

	return nil
}

// downloadPhoto 下载照片（快速失败 + 异步重试）
func (s *PhotoSyncer) downloadPhoto(ctx context.Context, photoUrl, destDir, fileName string, photoID, orderID uint64) error {
	destPath := filepath.Join(destDir, fileName)
	currentTimeout := s.calculateTimeout(0)

	logx.Infof("下载照片到: %s, 超时时间: %v", destPath, currentTimeout)

	// 第一次尝试
	err := s.downloadPhotoOnce(ctx, photoUrl, destPath, currentTimeout)
	if err == nil {
		return nil
	}

	// 第一次失败，判断是否应该异步重试
	if s.config.AsyncRetry && s.shouldRetry(err, 0) {
		// 加入异步重试队列
		task := &RetryTask{
			PhotoID:  photoID,
			URL:      photoUrl,
			DestDir:  destDir,
			FileName: fileName,
			Attempt:  1,
			OrderID:  orderID,
		}

		select {
		case s.retryQueue <- task:
			logx.Infof("照片加入异步重试队列: ID: %d, URL: %s", photoID, photoUrl)
			return fmt.Errorf("照片下载失败，已加入重试队列: %v", err)
		default:
			logx.Errorf("重试队列已满，无法加入重试: ID: %d", photoID)
			return fmt.Errorf("照片下载失败且重试队列已满: %v", err)
		}
	}

	// 不启用异步重试或不应该重试的错误
	return fmt.Errorf("照片下载失败: %v", err)
}

// downloadPhotoOnce 执行一次照片下载
func (s *PhotoSyncer) downloadPhotoOnce(ctx context.Context, photoUrl, destPath string, timeout time.Duration) error {
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
		os.Remove(tempPath) // 清理临时文件
		return fmt.Errorf("保存文件内容失败: %v", err)
	}

	// 关闭文件
	out.Close()

	// 原子性重命名，避免部分写入的文件
	if err := os.Rename(tempPath, destPath); err != nil {
		os.Remove(tempPath) // 清理临时文件
		return fmt.Errorf("重命名文件失败: %v", err)
	}

	return nil
}

// calculateTimeout 计算当前尝试的超时时间
func (s *PhotoSyncer) calculateTimeout(attempt int) time.Duration {
	// 基础超时时间随重试次数递增：30s -> 60s -> 90s -> 120s
	timeout := s.config.DownloadTimeout * (attempt + 1)
	if timeout > s.config.MaxDownloadTimeout {
		timeout = s.config.MaxDownloadTimeout
	}
	return time.Duration(timeout) * time.Second
}

// calculateRetryDelay 计算重试延迟时间（指数退避）
func (s *PhotoSyncer) calculateRetryDelay(attempt int) time.Duration {
	// 指数退避：2s -> 4s -> 8s
	delay := s.config.RetryBaseDelay * (1 << (attempt - 1))
	if delay > 10 {
		delay = 10 // 最大延迟10秒
	}
	return time.Duration(delay) * time.Second
}

// shouldRetry 判断是否应该重试
func (s *PhotoSyncer) shouldRetry(err error, attempt int) bool {
	// 已达到最大重试次数
	if attempt >= s.config.MaxRetries {
		return false
	}

	errorStr := err.Error()

	// 可重试的错误类型
	retryableErrors := []string{
		"context deadline exceeded",
		"Client.Timeout exceeded",
		"connection reset by peer",
		"connection refused",
		"no such host",
		"network is unreachable",
		"i/o timeout",
		"EOF",
	}

	for _, retryableError := range retryableErrors {
		if strings.Contains(errorStr, retryableError) {
			return true
		}
	}

	// HTTP 5xx 错误通常是服务器临时问题，可以重试
	if strings.Contains(errorStr, "HTTP响应状态码不是200: 5") {
		return true
	}

	// 其他错误不重试（如404, 403等）
	return false
}

// retryWorker 异步重试工作协程
func (s *PhotoSyncer) retryWorker(workerID int) {
	logx.Infof("异步重试工作协程 %d 启动", workerID)
	defer logx.Infof("异步重试工作协程 %d 停止", workerID)

	for {
		select {
		case <-s.ctx.Done():
			return
		case task, ok := <-s.retryQueue:
			if !ok {
				return // 队列已关闭
			}
			s.processRetryTask(task, workerID)
		}
	}
}

// processRetryTask 处理单个重试任务
func (s *PhotoSyncer) processRetryTask(task *RetryTask, workerID int) {
	if task.Attempt > s.config.MaxRetries {
		logx.Errorf("工作协程 %d: 照片 %d 已达到最大重试次数，放弃重试", workerID, task.PhotoID)
		s.updatePhotoStatus(s.ctx, task.PhotoID, model.PhotoStatusFailed, "达到最大重试次数")
		return
	}

	// 重试前延迟
	retryDelay := s.calculateRetryDelay(task.Attempt)
	logx.Infof("工作协程 %d: 照片 %d 第 %d 次重试，延迟 %v", workerID, task.PhotoID, task.Attempt, retryDelay)

	select {
	case <-s.ctx.Done():
		return
	case <-time.After(retryDelay):
		// 继续执行重试
	}

	// 计算当前尝试的超时时间
	currentTimeout := s.calculateTimeout(task.Attempt)
	destPath := filepath.Join(task.DestDir, task.FileName)

	err := s.downloadPhotoOnce(s.ctx, task.URL, destPath, currentTimeout)
	if err == nil {
		// 重试成功
		logx.Infof("工作协程 %d: 照片 %d 重试成功，共重试 %d 次", workerID, task.PhotoID, task.Attempt)
		s.updatePhotoStatus(s.ctx, task.PhotoID, model.PhotoStatusSuccess, "")

		// 检查所属订单是否可以完成
		s.checkAndCompleteOrder(s.ctx, task.OrderID)
		return
	}

	// 重试失败，判断是否继续重试
	if s.shouldRetry(err, task.Attempt) {
		logx.Infof("工作协程 %d: 照片 %d 第 %d 次重试失败: %v", workerID, task.PhotoID, task.Attempt, err)

		// 继续重试
		task.Attempt++
		select {
		case s.retryQueue <- task:
			// 成功加入队列继续重试
		default:
			// 队列满了，标记为失败
			logx.Errorf("工作协程 %d: 重试队列已满，照片 %d 标记为失败", workerID, task.PhotoID)
			s.updatePhotoStatus(s.ctx, task.PhotoID, model.PhotoStatusFailed, fmt.Sprintf("重试失败: %v", err))
		}
	} else {
		// 不应该重试的错误
		logx.Errorf("工作协程 %d: 照片 %d 重试失败且不应继续重试: %v", workerID, task.PhotoID, err)
		s.updatePhotoStatus(s.ctx, task.PhotoID, model.PhotoStatusFailed, fmt.Sprintf("重试失败: %v", err))
	}
}

// isTimeoutError 判断是否为超时错误
func (s *PhotoSyncer) isTimeoutError(err error) bool {
	errorStr := err.Error()
	timeoutIndicators := []string{
		"context deadline exceeded",
		"Client.Timeout exceeded",
		"i/o timeout",
	}

	for _, indicator := range timeoutIndicators {
		if strings.Contains(errorStr, indicator) {
			return true
		}
	}
	return false
}

// 添加一个工具函数来获取没有查询参数的文件名
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

// cleanOrderDirectory 清空订单目录中的所有内容
func (s *PhotoSyncer) cleanOrderDirectory(orderDir string) error {
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
				return fmt.Errorf("删除目录失败: %v", err)
			}
		} else {
			if err := os.Remove(filePath); err != nil {
				return fmt.Errorf("删除文件失败: %v", err)
			}
		}
	}

	return nil
}
