package sync

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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

// SyncPhotos 执行照片同步操作，扫描待处理订单并创建目录结构
func (s *PhotoSyncer) SyncPhotos(ctx context.Context) error {
	logx.Info("开始同步照片...")

	// 检查输出目录是否存在，不存在则创建
	if err := os.MkdirAll(s.config.OutputPath, 0755); err != nil {
		logx.Errorf("无法创建输出目录: %v", err)
		return err
	}

	// 首先处理失败的照片，进行重试
	if err := s.retryFailedPhotos(ctx); err != nil {
		logx.Errorf("重试失败照片时出错: %v", err)
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
		// 更新订单状态为处理中
		if err := s.orderModel.UpdateStatus(ctx, order.Id, model.OrderStatusProcessing); err != nil {
			logx.Errorf("更新订单状态失败, 订单ID: %d, 错误: %v", order.Id, err)
			continue
		}

		// 处理订单照片
		err := s.processOrderPhotos(ctx, order)
		if err != nil {
			logx.Errorf("处理订单照片失败, 订单ID: %d, 错误: %v", order.Id, err)
			// 出错时将订单状态还原
			_ = s.orderModel.UpdateStatus(ctx, order.Id, model.OrderStatusPending)
			continue
		}

		// 更新订单状态为已完成
		if err := s.orderModel.UpdateStatus(ctx, order.Id, model.OrderStatusCompleted); err != nil {
			logx.Errorf("更新订单状态失败, 订单ID: %d, 错误: %v", order.Id, err)
		} else {
			logx.Infof("订单处理完成, 订单ID: %d, 订单号: %s", order.Id, order.OrderSn)
		}
	}

	logx.Info("照片同步完成")
	return nil
}

// retryFailedPhotos 重试下载失败的照片
func (s *PhotoSyncer) retryFailedPhotos(ctx context.Context) error {
	failedPhotos, err := s.photoModel.FindFailedPhotos(ctx, s.config.BatchSize)
	if err != nil {
		return fmt.Errorf("查询失败照片出错: %v", err)
	}

	if len(failedPhotos) == 0 {
		logx.Info("没有找到需要重试的失败照片")
		return nil
	}

	logx.Infof("找到 %d 张失败照片需要重试", len(failedPhotos))

	// 按订单ID分组
	photosByOrder := make(map[uint64][]*model.Photo)
	for _, photo := range failedPhotos {
		photosByOrder[photo.OrderId] = append(photosByOrder[photo.OrderId], photo)
	}

	// 处理每个订单的失败照片
	var successCount, failCount int
	for orderId, photos := range photosByOrder {
		// 查询订单信息
		order, err := s.orderModel.FindOne(ctx, orderId)
		if err != nil {
			logx.Errorf("查询订单信息失败, 订单ID: %d, 错误: %v", orderId, err)
			continue
		}

		// 创建目录结构
		orderDir, err := s.createOrderDirectories(order)
		if err != nil {
			logx.Errorf("为订单创建目录失败, 订单ID: %d, 错误: %v", orderId, err)
			continue
		}

		// 按照照片的size和unit分组
		sizeDirs := make(map[string]string)

		// 重试下载每张照片
		for _, photo := range photos {
			// 生成尺寸目录名 (例如: "3inch")
			sizeDir := fmt.Sprintf("%d%s", photo.Size, photo.Unit)

			// 检查该尺寸的目录是否已创建
			if _, exists := sizeDirs[sizeDir]; !exists {
				// 创建尺寸目录
				fullSizeDir := filepath.Join(orderDir, sizeDir)
				if err := os.MkdirAll(fullSizeDir, 0755); err != nil {
					errMsg := fmt.Sprintf("创建尺寸目录失败 %s: %v", sizeDir, err)
					logx.Error(errMsg)
					// 更新照片状态为失败
					if updateErr := s.photoModel.UpdateStatus(ctx, photo.Id, model.PhotoStatusFailed, errMsg); updateErr != nil {
						logx.Errorf("更新照片状态失败, 照片ID: %d, 错误: %v", photo.Id, updateErr)
					}
					failCount++
					continue
				}
				sizeDirs[sizeDir] = fullSizeDir
			}

			// 下载照片到对应的目录
			destDir := sizeDirs[sizeDir]
			fileName := filepath.Base(photo.Url)

			if err := s.downloadPhoto(ctx, photo.Url, destDir, fileName); err != nil {
				errMsg := fmt.Sprintf("下载照片失败: %v", err)
				logx.Errorf("重试下载照片失败, 照片ID: %d, URL: %s, 错误: %v", photo.Id, photo.Url, err)
				// 更新照片状态为失败，并记录错误信息
				if updateErr := s.photoModel.UpdateStatus(ctx, photo.Id, model.PhotoStatusFailed, errMsg); updateErr != nil {
					logx.Errorf("更新照片状态失败, 照片ID: %d, 错误: %v", photo.Id, updateErr)
				}
				failCount++
				continue
			}

			// 更新照片状态为成功
			if err := s.photoModel.UpdateStatus(ctx, photo.Id, model.PhotoStatusSuccess, ""); err != nil {
				logx.Errorf("更新照片状态失败, 照片ID: %d, 错误: %v", photo.Id, err)
			}

			successCount++
		}
	}

	logx.Infof("重试完成, 成功: %d, 失败: %d", successCount, failCount)
	return nil
}

// createOrderDirectories 创建订单的目录结构，返回订单目录路径
func (s *PhotoSyncer) createOrderDirectories(order *model.Order) (string, error) {
	// 创建以日期为名的目录
	today := time.Now().Format("2006-01-02")
	dateDir := filepath.Join(s.config.OutputPath, today)
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		return "", fmt.Errorf("创建日期目录失败: %v", err)
	}

	// 创建以收货人姓名-订单号为名的目录
	orderDir := filepath.Join(dateDir, fmt.Sprintf("%s-%s", order.Receiver, order.OrderSn))
	if err := os.MkdirAll(orderDir, 0755); err != nil {
		return "", fmt.Errorf("创建订单目录失败: %v", err)
	}

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

	// 按照照片的size和unit分组
	sizeDirs := make(map[string]string)

	// 处理每张照片
	var successCount, failCount int

	for _, photo := range photos {
		// 生成尺寸目录名 (例如: "3inch")
		sizeDir := fmt.Sprintf("%d%s", photo.Size, photo.Unit)

		// 检查该尺寸的目录是否已创建
		if _, exists := sizeDirs[sizeDir]; !exists {
			// 创建尺寸目录
			fullSizeDir := filepath.Join(orderDir, sizeDir)
			if err := os.MkdirAll(fullSizeDir, 0755); err != nil {
				errMsg := fmt.Sprintf("创建尺寸目录失败 %s: %v", sizeDir, err)
				logx.Error(errMsg)
				// 更新照片状态为失败
				if updateErr := s.photoModel.UpdateStatus(ctx, photo.Id, model.PhotoStatusFailed, errMsg); updateErr != nil {
					logx.Errorf("更新照片状态失败, 照片ID: %d, 错误: %v", photo.Id, updateErr)
				}
				failCount++
				continue
			}
			sizeDirs[sizeDir] = fullSizeDir
		}

		// 下载照片到对应的目录
		destDir := sizeDirs[sizeDir]
		fileName := filepath.Base(photo.Url)

		if err := s.downloadPhoto(ctx, photo.Url, destDir, fileName); err != nil {
			errMsg := fmt.Sprintf("下载照片失败: %v", err)
			logx.Errorf("下载照片失败, 照片ID: %d, URL: %s, 错误: %v", photo.Id, photo.Url, err)
			// 更新照片状态为失败，并记录错误信息
			if updateErr := s.photoModel.UpdateStatus(ctx, photo.Id, model.PhotoStatusFailed, errMsg); updateErr != nil {
				logx.Errorf("更新照片状态失败, 照片ID: %d, 错误: %v", photo.Id, updateErr)
			}
			failCount++
			continue
		}

		// 更新照片状态为成功
		if err := s.photoModel.UpdateStatus(ctx, photo.Id, model.PhotoStatusSuccess, ""); err != nil {
			logx.Errorf("更新照片状态失败, 照片ID: %d, 错误: %v", photo.Id, err)
		}

		successCount++
	}

	logx.Infof("订单 %d 处理完成, 成功: %d, 失败: %d", order.Id, successCount, failCount)

	return nil
}

// downloadPhoto 下载照片
func (s *PhotoSyncer) downloadPhoto(ctx context.Context, photoUrl, destDir, fileName string) error {
	// 创建完整的目标文件路径
	destPath := filepath.Join(destDir, fileName)

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
		Timeout: time.Duration(30) * time.Second,
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

	// 创建目标文件
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %v", err)
	}
	defer out.Close()

	// 复制内容
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("保存文件内容失败: %v", err)
	}

	return nil
}
