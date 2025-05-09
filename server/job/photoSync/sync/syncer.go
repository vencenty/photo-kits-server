package sync

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"photo-kits-server/server/job/photoSync/config"
	"photo-kits-server/server/model"
)

// PhotoSyncer 照片同步器
type PhotoSyncer struct {
	photoModel model.PhotoModel
	config     config.SyncConfig
}

// NewPhotoSyncer 创建一个新的照片同步器
func NewPhotoSyncer(photoModel model.PhotoModel, config config.SyncConfig) *PhotoSyncer {
	return &PhotoSyncer{
		photoModel: photoModel,
		config:     config,
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

	// TODO: 实现实际的同步逻辑
	// 1. 扫描源目录中的照片文件
	// 2. 处理照片元数据
	// 3. 根据需要将照片保存到数据库
	// 4. 移动已处理的照片到备份目录

	// 这里是示例代码，实际实现需要根据具体需求开发
	files, err := s.scanSourceDirectory()
	if err != nil {
		return err
	}

	for _, file := range files {
		// 处理每个文件
		logx.Infof("处理文件: %s", file)

		// 在这里添加实际的处理逻辑
	}

	logx.Info("照片同步完成")
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

	// 保存到数据库
	_, err = s.photoModel.Insert(ctx, photo)
	if err != nil {
		return err
	}

	// 将文件移动到备份目录
	backupPath := filepath.Join(s.config.BackupPath, strconv.FormatInt(time.Now().Unix(), 10)+"_"+filepath.Base(filePath))
	return os.Rename(filePath, backupPath)
}
