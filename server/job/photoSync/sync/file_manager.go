package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"server/model"
)

// FileManager 文件管理器
type FileManager struct {
	outputPath string
}

// NewFileManager 创建文件管理器
func NewFileManager(outputPath string) *FileManager {
	return &FileManager{
		outputPath: outputPath,
	}
}

// CreateSpecBasedDirectories 创建基于规格的目录结构
// 新目录结构: 输出根目录/年份/年月/年月日/规格/订单号-收货人/比例/
func (fm *FileManager) CreateSpecBasedDirectories(order *model.Order, spec, aspectCategory string) (string, error) {
	baseTime := order.CreatedAt

	// 创建年份目录（如："2025"）
	yearName := fmt.Sprintf("%d", baseTime.Year())
	yearDir := filepath.Join(fm.outputPath, yearName)
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

	// 创建规格目录（如："5寸-满版"）
	if spec == "" {
		spec = "默认规格"
	}
	specDir := filepath.Join(dateDir, spec)
	if err := os.MkdirAll(specDir, 0755); err != nil {
		return "", fmt.Errorf("创建规格目录失败: %v", err)
	}

	// 创建订单目录（如："ORDER123-张三"）
	orderDirName := fmt.Sprintf("%s-%s", order.OrderSn, order.Receiver)
	orderDir := filepath.Join(specDir, orderDirName)
	if err := os.MkdirAll(orderDir, 0755); err != nil {
		return "", fmt.Errorf("创建订单目录失败: %v", err)
	}

	// 创建比例目录（如："4_3"）- 最后一层
	finalDir := filepath.Join(orderDir, aspectCategory)
	if err := os.MkdirAll(finalDir, 0755); err != nil {
		return "", fmt.Errorf("创建比例目录失败: %v", err)
	}

	logx.Infof("为订单 %s 创建目录: %s (规格: %s, 比例: %s, 基于时间: %s)",
		order.OrderSn, finalDir, spec, aspectCategory, baseTime.Format("2006-01-02 15:04:05"))

	return finalDir, nil
}

// GenerateUniqueFileName 生成唯一的文件名，避免重复
func (fm *FileManager) GenerateUniqueFileName(dir, fileName string, usedNames map[string]bool) string {
	// 检查原始文件名是否已使用
	if !usedNames[fileName] && !fm.FileExists(filepath.Join(dir, fileName)) {
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
		if !usedNames[newFileName] && !fm.FileExists(filepath.Join(dir, newFileName)) {
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

// FileExists 检查文件是否存在
func (fm *FileManager) FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

// CleanDirectory 清空指定目录中的所有内容
func (fm *FileManager) CleanDirectory(dirPath string) error {
	// 检查目录是否存在
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		logx.Infof("目录不存在，无需清空: %s", dirPath)
		return nil
	}

	logx.Infof("开始清空目录: %s", dirPath)

	// 打开目录
	dir, err := os.Open(dirPath)
	if err != nil {
		return fmt.Errorf("打开目录失败: %v", err)
	}
	defer dir.Close()

	// 读取目录中的所有文件和子目录
	files, err := dir.Readdir(0)
	if err != nil {
		return fmt.Errorf("读取目录内容失败: %v", err)
	}

	// 遍历所有文件和子目录，删除它们
	for _, file := range files {
		filePath := filepath.Join(dirPath, file.Name())

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

	logx.Infof("目录清空完成: %s", dirPath)
	return nil
}

// EnsureOutputDirectory 确保输出根目录存在
func (fm *FileManager) EnsureOutputDirectory() error {
	if err := os.MkdirAll(fm.outputPath, 0755); err != nil {
		return fmt.Errorf("无法创建输出目录: %v", err)
	}
	return nil
}
