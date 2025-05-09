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

	// 创建以日期为名的目录
	today := time.Now().Format("2006-01-02")
	dateDir := filepath.Join(s.config.OutputPath, today)
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		return fmt.Errorf("创建日期目录失败: %v", err)
	}

	// 创建以收货人姓名-订单号为名的目录
	orderDir := filepath.Join(dateDir, fmt.Sprintf("%s-%s", order.Receiver, order.OrderSn))
	if err := os.MkdirAll(orderDir, 0755); err != nil {
		return fmt.Errorf("创建订单目录失败: %v", err)
	}

	// 创建不同尺寸的子目录
	sizeMap := make(map[string]string)
	for _, sizeType := range s.config.SizeTypes {
		sizeDir := filepath.Join(orderDir, sizeType.Name)
		if err := os.MkdirAll(sizeDir, 0755); err != nil {
			return fmt.Errorf("创建尺寸目录失败: %v", err)
		}
		sizeMap[sizeType.Name] = sizeDir
	}

	// 如果没有配置尺寸，创建一个默认目录
	if len(s.config.SizeTypes) == 0 {
		defaultDir := filepath.Join(orderDir, "默认尺寸")
		if err := os.MkdirAll(defaultDir, 0755); err != nil {
			return fmt.Errorf("创建默认尺寸目录失败: %v", err)
		}
		sizeMap["默认尺寸"] = defaultDir
	}

	// 处理每张照片
	var successCount, failCount int
	for _, photo := range photos {
		// 从URL下载照片
		// 这里假设照片的尺寸信息包含在Unit字段中，例如"3寸"、"4寸"等
		// 如果不是，您需要根据实际情况修改这部分逻辑
		sizeType := photo.Unit
		if sizeDir, ok := sizeMap[sizeType]; ok {
			// 使用对应尺寸的目录
			if err := s.downloadPhoto(ctx, photo.Url, sizeDir, filepath.Base(photo.Url)); err != nil {
				logx.Errorf("下载照片失败, 照片ID: %d, URL: %s, 错误: %v", photo.Id, photo.Url, err)
				failCount++
				continue
			}
		} else {
			// 使用默认尺寸的目录
			defaultDir := sizeMap["默认尺寸"]
			if defaultDir == "" && len(sizeMap) > 0 {
				// 如果没有默认尺寸目录但有其他尺寸目录，使用第一个
				for _, dir := range sizeMap {
					defaultDir = dir
					break
				}
			}
			if err := s.downloadPhoto(ctx, photo.Url, defaultDir, filepath.Base(photo.Url)); err != nil {
				logx.Errorf("下载照片失败, 照片ID: %d, URL: %s, 错误: %v", photo.Id, photo.Url, err)
				failCount++
				continue
			}
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
