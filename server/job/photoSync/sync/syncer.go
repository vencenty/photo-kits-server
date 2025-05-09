package sync

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
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
		orderModel: model.NewOrderModel(db), // 如果需要处理订单，添加orderModel
	}
}

// SyncPhotos 执行照片同步操作
func (s *PhotoSyncer) SyncPhotos(ctx context.Context) error {
	logx.Info("开始同步照片...")

	// 检查源目录是否存在
	if _, err := os.Stat(s.config.SourcePath); os.IsNotExist(err) {
		logx.Errorf("源目录不存在: %s", s.config.SourcePath)
		return err
	}

	// 创建备份目录（如果不存在）
	if err := os.MkdirAll(s.config.BackupPath, 0755); err != nil {
		logx.Errorf("无法创建备份目录: %v", err)
		return err
	}

	// 扫描源目录中的照片文件
	files, err := s.scanSourceDirectory()
	if err != nil {
		logx.Errorf("扫描源目录失败: %v", err)
		return err
	}

	logx.Infof("找到 %d 个照片文件需要处理", len(files))

	if len(files) == 0 {
		logx.Info("没有找到需要处理的照片文件")
		return nil
	}

	// 按批次处理文件，每批次处理s.config.BatchSize个文件
	// 这里只是示例，实际业务逻辑需要根据具体需求调整
	// 例如：可以根据文件名或目录结构识别照片所属的订单ID
	//
	// 示例：假设所有照片都属于同一个订单（orderId = 1）
	// 实际应用中，您需要根据业务逻辑确定每个照片的orderId

	var processedCount int
	batchSize := s.config.BatchSize

	// 模拟一个订单ID，实际应用中需要根据业务逻辑获取
	// 例如：可以从文件名、目录名等提取订单ID
	var sampleOrderId uint64 = 1

	for i := 0; i < len(files); i += batchSize {
		end := i + batchSize
		if end > len(files) {
			end = len(files)
		}

		batchFiles := files[i:end]
		logx.Infof("处理第 %d 批照片，共 %d 张", i/batchSize+1, len(batchFiles))

		// 处理这一批照片
		if err := s.BatchProcessPhotos(ctx, batchFiles, sampleOrderId); err != nil {
			logx.Errorf("处理照片批次失败: %v", err)
			return err
		}

		processedCount += len(batchFiles)
		logx.Infof("已成功处理 %d/%d 张照片", processedCount, len(files))
	}

	logx.Infof("照片同步完成，共处理 %d 张照片", processedCount)
	return nil
}

// scanSourceDirectory 扫描源目录中的照片文件
func (s *PhotoSyncer) scanSourceDirectory() ([]string, error) {
	var files []string

	err := filepath.Walk(s.config.SourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录
		if info.IsDir() {
			return nil
		}

		// 只处理图片文件（可以根据实际需求调整）
		ext := filepath.Ext(path)
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// processPhoto 处理单个照片
func (s *PhotoSyncer) processPhoto(ctx context.Context, filePath string, orderId uint64) error {
	// 获取文件信息
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	// 创建照片记录
	photo := &model.Photo{
		OrderId: orderId,
		Url:     filepath.Base(filePath),
		Size:    fileInfo.Size(),
		Unit:    "B",
	}

	// 使用photoModel保存到数据库
	_, err = s.photoModel.Insert(ctx, photo)
	if err != nil {
		logx.Errorf("保存照片记录失败: %v", err)
		return err
	}

	// 将文件移动到备份目录
	backupPath := filepath.Join(s.config.BackupPath, strconv.FormatInt(time.Now().Unix(), 10)+"_"+filepath.Base(filePath))
	return os.Rename(filePath, backupPath)
}

// BatchProcessPhotos 批量处理照片，可以在事务中执行
func (s *PhotoSyncer) BatchProcessPhotos(ctx context.Context, files []string, orderId uint64) error {
	// 这里可以使用事务处理批量插入
	// 例如：
	// session := sqlx.NewSessionFromConn(s.db) // 创建会话
	// session.Begin()  // 开始事务
	// ... 批量处理逻辑 ...
	// session.Commit() // 提交事务

	for _, file := range files {
		if err := s.processPhoto(ctx, file, orderId); err != nil {
			return err
		}
	}

	return nil
}
