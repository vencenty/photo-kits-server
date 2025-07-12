package main

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// 照片分类器
type PhotoClassifier struct {
	SourceDir string
	OutputDir string
}

// 支持的图片格式
var supportedFormats = []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".heic"}

// 分类文件夹映射
var aspectRatioFolders = map[string]string{
	"1:1":   "1_1",
	"5:7":   "5_7",
	"2:3":   "2_3",
	"4:3":   "4_3",
	"other": "other",
}

// 检查文件是否为支持的图片格式
func isImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	for _, format := range supportedFormats {
		if ext == format {
			return true
		}
	}
	return false
}

// 获取图片尺寸
func getImageDimensions(filePath string) (width, height int, err error) {
	// 检查是否为HEIC文件
	if strings.ToLower(filepath.Ext(filePath)) == ".heic" {
		return getHeicDimensions(filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}

	return config.Width, config.Height, nil
}

// 获取HEIC文件尺寸（使用系统命令）
func getHeicDimensions(filePath string) (width, height int, err error) {
	// 创建临时JPG文件来获取尺寸
	tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("temp_%d.jpg", os.Getpid()))
	defer os.Remove(tempFile)

	// 转换HEIC为临时JPG
	if err := convertHeicToJpgWithSips(filePath, tempFile); err != nil {
		return 0, 0, fmt.Errorf("转换HEIC失败: %v", err)
	}

	// 从临时JPG文件获取尺寸
	file, err := os.Open(tempFile)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}

	return config.Width, config.Height, nil
}

// 使用系统命令将HEIC转换为JPG
func convertHeicToJpgWithSips(heicPath, jpgPath string) error {
	switch runtime.GOOS {
	case "darwin":
		// macOS 使用 sips 命令
		cmd := exec.Command("sips", "-s", "format", "jpeg", heicPath, "--out", jpgPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("sips转换失败: %v", err)
		}
	case "linux":
		// Linux 可以尝试使用 ImageMagick 的 convert 命令
		cmd := exec.Command("convert", heicPath, jpgPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("convert转换失败: %v (请确保安装了ImageMagick)", err)
		}
	case "windows":
		// Windows 系统提示用户手动转换
		return fmt.Errorf("Windows系统不支持自动HEIC转换，请手动转换HEIC文件为JPG后再运行")
	default:
		return fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}

	return nil
}

// 将HEIC文件转换为JPG
func convertHeicToJpg(heicPath, jpgPath string) error {
	if err := convertHeicToJpgWithSips(heicPath, jpgPath); err != nil {
		return err
	}
	fmt.Printf("HEIC转JPG成功: %s -> %s\n", heicPath, jpgPath)
	return nil
}

// 计算长宽比并分类
func classifyAspectRatio(width, height int) string {
	// 计算比例，取较小的值作为分母
	var ratio float64
	if width > height {
		ratio = float64(width) / float64(height)
	} else {
		ratio = float64(height) / float64(width)
	}

	// 允许的误差范围
	tolerance := 0.05

	// 检查各种比例
	ratios := map[string]float64{
		"1:1": 1.0,       // 1:1
		"5:7": 7.0 / 5.0, // 5:7 = 1.4
		"2:3": 3.0 / 2.0, // 2:3 = 1.5
		"4:3": 4.0 / 3.0, // 4:3 = 1.33
	}

	for category, expectedRatio := range ratios {
		if math.Abs(ratio-expectedRatio) <= tolerance {
			return category
		}
	}

	return "other"
}

// 创建目录
func createDirectoryIfNotExists(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("创建目录失败: %v", err)
		}
		fmt.Printf("创建目录: %s\n", dir)
	}
	return nil
}

// 复制文件
func copyFile(src, dst string) error {
	data, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dst, data, 0644)
}

// 分类照片
func (pc *PhotoClassifier) ClassifyPhotos() error {
	// 创建输出目录
	if err := createDirectoryIfNotExists(pc.OutputDir); err != nil {
		return err
	}

	// 创建分类子目录
	for _, folder := range aspectRatioFolders {
		subDir := filepath.Join(pc.OutputDir, folder)
		if err := createDirectoryIfNotExists(subDir); err != nil {
			return err
		}
	}

	// 读取源目录中的所有文件
	files, err := ioutil.ReadDir(pc.SourceDir)
	if err != nil {
		return fmt.Errorf("读取源目录失败: %v", err)
	}

	stats := make(map[string]int)
	var totalImages int

	// 遍历所有文件
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := file.Name()
		if !isImageFile(filename) {
			continue
		}

		totalImages++
		srcPath := filepath.Join(pc.SourceDir, filename)

		// 获取图片尺寸
		width, height, err := getImageDimensions(srcPath)
		if err != nil {
			log.Printf("无法读取图片尺寸 %s: %v", filename, err)
			continue
		}

		// 分类
		category := classifyAspectRatio(width, height)
		stats[category]++

		// 确定最终文件名和路径
		var finalFilename string
		if strings.ToLower(filepath.Ext(filename)) == ".heic" {
			// HEIC文件转换为JPG格式保存
			finalFilename = strings.TrimSuffix(filename, filepath.Ext(filename)) + ".jpg"
		} else {
			finalFilename = filename
		}

		// 复制到对应文件夹
		dstDir := filepath.Join(pc.OutputDir, aspectRatioFolders[category])
		dstPath := filepath.Join(dstDir, finalFilename)

		// 处理HEIC文件：转换为JPG后保存
		if strings.ToLower(filepath.Ext(filename)) == ".heic" {
			if err := convertHeicToJpg(srcPath, dstPath); err != nil {
				log.Printf("HEIC转JPG失败 %s: %v", filename, err)
				continue
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				log.Printf("复制文件失败 %s: %v", finalFilename, err)
				continue
			}
		}

		fmt.Printf("分类: %s -> %s (尺寸: %dx%d)\n", finalFilename, category, width, height)
	}

	// 输出统计信息
	fmt.Printf("\n=== 分类统计 ===\n")
	fmt.Printf("总图片数: %d\n", totalImages)
	for category, count := range stats {
		fmt.Printf("%s: %d 张\n", category, count)
	}

	return nil
}

func main() {
	// 检查命令行参数
	if len(os.Args) < 3 {
		fmt.Println("使用方法: go run main.go <源目录> <输出目录>")
		fmt.Println("示例: go run main.go ./photos ./classified")
		os.Exit(1)
	}

	sourceDir := os.Args[1]
	outputDir := os.Args[2]

	// 检查源目录是否存在
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		log.Fatalf("源目录不存在: %s", sourceDir)
	}

	// 创建照片分类器
	classifier := &PhotoClassifier{
		SourceDir: sourceDir,
		OutputDir: outputDir,
	}

	// 开始分类
	fmt.Printf("开始分类照片...\n")
	fmt.Printf("源目录: %s\n", sourceDir)
	fmt.Printf("输出目录: %s\n", outputDir)
	fmt.Printf("支持的格式: %s\n", strings.Join(supportedFormats, ", "))
	fmt.Printf("分类比例: 1:1, 5:7, 2:3, 4:3, other\n")
	fmt.Println("----------------------------------------")

	if err := classifier.ClassifyPhotos(); err != nil {
		log.Fatalf("分类失败: %v", err)
	}

	fmt.Println("分类完成!")
}
