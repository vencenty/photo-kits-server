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
// 注意：在调用此方法前，应先调用 ConvertToJPG 将特殊格式转换为 JPG
func (ia *ImageAnalyzer) GetImageDimensions(imagePath string) (width, height int, err error) {
	// 首先尝试使用 Go 标准库解析（支持 jpg, png, gif）
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

	// 如果标准库解析失败，尝试使用外部工具（ImageMagick, exiftool, ffprobe）
	logx.Errorf("Go标准库解析图片失败: %v，尝试使用外部工具", err)
	width, height, err = ia.getDimensionsWithExternalTools(imagePath)
	if err == nil {
		logx.Infof("成功通过外部工具获取图片尺寸: %dx%d", width, height)
		return width, height, nil
	}

	// 所有方法都失败，使用默认尺寸
	logx.Errorf("无法获取图片尺寸: %v，使用默认尺寸继续处理", err)
	width, height = ia.getDefaultDimensions()
	return width, height, nil
}

// getDimensionsWithExternalTools 尝试使用外部工具获取图片尺寸（适用于各种格式）
func (ia *ImageAnalyzer) getDimensionsWithExternalTools(imagePath string) (width, height int, err error) {
	// 尝试使用 ImageMagick 的 identify 命令（最常用，支持最多格式）
	if width, height, err := ia.tryImageMagick(imagePath); err == nil {
		return width, height, nil
	}

	// 尝试使用 exiftool 获取尺寸信息
	if width, height, err := ia.tryExifTool(imagePath); err == nil {
		return width, height, nil
	}

	// 尝试使用 ffprobe（适合多媒体文件）
	if width, height, err := ia.tryFFProbe(imagePath); err == nil {
		return width, height, nil
	}

	return 0, 0, fmt.Errorf("所有外部工具都无法获取图片尺寸")
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

// ConvertToJPG 将特殊格式的图片转换为标准JPG格式
// 支持的转换格式：heic, heif, webp, jfif
// 如果文件已经是jpg格式，则不进行转换直接返回原路径
// 如果需要转换，会生成一个新的jpg文件，并删除原文件，返回新文件路径
func (ia *ImageAnalyzer) ConvertToJPG(imagePath string) (newPath string, converted bool, err error) {
	fileExt := strings.ToLower(filepath.Ext(imagePath))

	// 检查是否需要转换
	needConvert := ia.needsConversion(fileExt)

	if !needConvert {
		// 不需要转换，直接返回原路径
		return imagePath, false, nil
	}

	// 生成新的jpg文件路径
	newPath = strings.TrimSuffix(imagePath, fileExt) + ".jpg"

	logx.Infof("检测到特殊格式 %s，开始转换为 JPG", fileExt)

	// 使用 ImageMagick 的 convert 命令转换（支持所有格式）
	err = ia.convertWithImageMagick(imagePath, newPath)
	if err == nil {
		// 转换成功，删除原文件
		if removeErr := os.Remove(imagePath); removeErr != nil {
			logx.Errorf("删除原文件失败: %s, 错误: %v", imagePath, removeErr)
		}
		logx.Infof("图片格式转换成功: %s -> JPG", fileExt)
		return newPath, true, nil
	}

	// ImageMagick 失败，记录错误但不中断流程
	logx.Errorf("格式转换失败: %v，将使用原文件继续处理", err)
	return imagePath, false, fmt.Errorf("ImageMagick转换失败: %v", err)
}

// needsConversion 判断文件扩展名是否需要转换为JPG
func (ia *ImageAnalyzer) needsConversion(fileExt string) bool {
	switch fileExt {
	case ".heic", ".heif":
		// Apple 的 HEIC/HEIF 格式
		return true
	case ".webp":
		// Google 的 WebP 格式
		return true
	case ".jfif":
		// JPEG 文件交换格式（JPEG File Interchange Format）
		return true
	case ".jpg", ".jpeg":
		// 已经是标准 JPG 格式
		return false
	default:
		// 其他格式保持不变（如 png, gif 等）
		return false
	}
}

// convertWithImageMagick 使用 ImageMagick 的 convert 命令转换图片
// 支持的格式：heic, heif, webp, jfif, png, gif 等
// 转换为 JPG 格式，质量设置为 95%
func (ia *ImageAnalyzer) convertWithImageMagick(srcPath, dstPath string) error {
	// convert 源文件 -quality 95 目标文件.jpg
	cmd := exec.Command("convert", srcPath, "-quality", "95", dstPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("convert命令执行失败: %v, 输出: %s", err, string(output))
	}

	// 验证输出文件是否存在
	if _, err := os.Stat(dstPath); os.IsNotExist(err) {
		return fmt.Errorf("转换后的文件不存在: %s", dstPath)
	}

	return nil
}
