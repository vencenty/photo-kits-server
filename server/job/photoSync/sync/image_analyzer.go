package sync

import (
	"fmt"
	"image"
	_ "image/gif"  // 支持GIF格式
	_ "image/jpeg" // 支持JPEG格式
	_ "image/png"  // 支持PNG格式
	"os"
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
	file, err := os.Open(imagePath)
	if err != nil {
		return 0, 0, fmt.Errorf("打开图片文件失败: %v", err)
	}
	defer file.Close()

	img, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, fmt.Errorf("解码图片配置失败: %v", err)
	}

	return img.Width, img.Height, nil
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
