package sync

import (
	"fmt"
	"image"
	_ "image/gif"  // 支持GIF格式
	_ "image/jpeg" // 支持JPEG格式
	_ "image/png"  // 支持PNG格式
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
)

// AspectRatio 宽高比结构
type AspectRatio struct {
	Name      string
	Ratio     float64
	Tolerance float64
}

// 预定义的标准宽高比（只保留常用比例）
var standardAspectRatios = []AspectRatio{
	{Name: "1_1", Ratio: 1.0, Tolerance: 0.02},
	{Name: "4_3", Ratio: 4.0 / 3.0, Tolerance: 0.02},
	{Name: "16_9", Ratio: 16.0 / 9.0, Tolerance: 0.02},
	{Name: "3_2", Ratio: 3.0 / 2.0, Tolerance: 0.02},
}

// ImageAnalyzer 图片分析器
type ImageAnalyzer struct{}

// NewImageAnalyzer 创建图片分析器
func NewImageAnalyzer() *ImageAnalyzer {
	return &ImageAnalyzer{}
}

// GetImageDimensions 获取图片的宽高
func (ia *ImageAnalyzer) GetImageDimensions(imagePath string) (width, height int, err error) {
	// 首先尝试标准的Go image库解析
	file, err := os.Open(imagePath)
	if err != nil {
		return 0, 0, fmt.Errorf("打开图片文件失败: %v", err)
	}
	defer file.Close()

	img, format, err := image.DecodeConfig(file)
	if err == nil {
		logx.Infof("成功解析图片尺寸，格式: %s, 尺寸: %dx%d", format, img.Width, img.Height)
		return img.Width, img.Height, nil
	}

	// 如果标准库失败，检查是否为HEIC格式
	fileExt := strings.ToLower(filepath.Ext(imagePath))
	if fileExt == ".heic" || fileExt == ".heif" {
		logx.Infof("检测到HEIC/HEIF格式文件，尝试使用替代方案获取尺寸: %s", imagePath)

		// 尝试使用外部工具获取HEIC文件尺寸
		width, height, err = ia.getHEICDimensions(imagePath)
		if err == nil {
			logx.Infof("成功通过外部工具获取HEIC尺寸: %dx%d", width, height)
			return width, height, nil
		}

		logx.Errorf("无法获取HEIC文件尺寸: %v", err)

		// HEIC解析失败，使用默认尺寸继续处理
		logx.Infof("HEIC文件尺寸获取失败，使用默认尺寸继续处理")
		width, height := ia.getDefaultDimensions()
		return width, height, nil
	}

	// 非HEIC格式但解析失败，尝试默认尺寸
	logx.Errorf("图片格式解析失败: %v，使用默认尺寸继续处理", err)
	width, height = ia.getDefaultDimensions()
	return width, height, nil
}

// getHEICDimensions 尝试使用外部工具获取HEIC文件尺寸
func (ia *ImageAnalyzer) getHEICDimensions(imagePath string) (width, height int, err error) {
	// 尝试使用exiftool获取尺寸信息
	if width, height, err := ia.tryExifTool(imagePath); err == nil {
		return width, height, nil
	}

	// 尝试使用ImageMagick的identify命令
	if width, height, err := ia.tryImageMagick(imagePath); err == nil {
		return width, height, nil
	}

	// 尝试使用ffprobe
	if width, height, err := ia.tryFFProbe(imagePath); err == nil {
		return width, height, nil
	}

	return 0, 0, fmt.Errorf("所有外部工具都无法获取HEIC文件尺寸")
}

// tryExifTool 尝试使用exiftool获取图片尺寸
func (ia *ImageAnalyzer) tryExifTool(imagePath string) (width, height int, err error) {
	cmd := exec.Command("exiftool", "-ImageWidth", "-ImageHeight", "-s", "-s", "-s", imagePath)
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) != 2 {
		return 0, 0, fmt.Errorf("exiftool输出格式不正确")
	}

	width, err = strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("解析宽度失败: %v", err)
	}

	height, err = strconv.Atoi(strings.TrimSpace(lines[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("解析高度失败: %v", err)
	}

	return width, height, nil
}

// tryImageMagick 尝试使用ImageMagick的identify命令获取图片尺寸
func (ia *ImageAnalyzer) tryImageMagick(imagePath string) (width, height int, err error) {
	cmd := exec.Command("identify", "-format", "%w %h", imagePath)
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}

	parts := strings.Fields(strings.TrimSpace(string(output)))
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("identify输出格式不正确")
	}

	width, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("解析宽度失败: %v", err)
	}

	height, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("解析高度失败: %v", err)
	}

	return width, height, nil
}

// tryFFProbe 尝试使用ffprobe获取图片尺寸
func (ia *ImageAnalyzer) tryFFProbe(imagePath string) (width, height int, err error) {
	cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "csv=p=0", "-select_streams", "v:0", "-show_entries", "stream=width,height", imagePath)
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}

	parts := strings.Split(strings.TrimSpace(string(output)), ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("ffprobe输出格式不正确")
	}

	width, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("解析宽度失败: %v", err)
	}

	height, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("解析高度失败: %v", err)
	}

	return width, height, nil
}

// getDefaultDimensions 返回默认的图片尺寸，并返回合理的宽高比
func (ia *ImageAnalyzer) getDefaultDimensions() (width, height int) {
	// 返回常见的手机照片尺寸，4:3比例
	return 4000, 3000
}

// CalculateAspectRatio 计算宽高比
func (ia *ImageAnalyzer) CalculateAspectRatio(width, height int) float64 {
	if height == 0 {
		return 0
	}
	return float64(width) / float64(height)
}

// GetAspectRatioCategory 根据宽高比获取分类名称
func (ia *ImageAnalyzer) GetAspectRatioCategory(ratio float64) string {
	// 标准化比例（取绝对值，因为3:4和4:3是同一个比例）
	standardRatio := ratio
	if ratio < 1 {
		standardRatio = 1 / ratio
	}

	// 查找匹配的标准比例
	for _, ar := range standardAspectRatios {
		if abs(standardRatio-ar.Ratio) <= ar.Tolerance {
			return ar.Name
		}
	}
	return "other"
}

// abs 计算绝对值
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
