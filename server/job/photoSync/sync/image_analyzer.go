package sync

import (
	"fmt"
	"image"
	_ "image/gif"  // 支持GIF格式
	_ "image/jpeg" // 支持JPEG格式
	_ "image/png"  // 支持PNG格式
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
)

// 常量定义
const (
	// JPG 转换质量（100 表示最高质量）
	jpegConversionQuality = "100"

	// 默认图片尺寸（4:3 比例，常见手机照片尺寸）
	defaultImageWidth  = 4000
	defaultImageHeight = 3000

	// 宽高比容差
	aspectRatioTolerance = 0.02
)

// AspectRatio 宽高比结构
type AspectRatio struct {
	Name      string  // 比例名称，如 "16_9"
	Ratio     float64 // 比例值，如 16.0/9.0
	Tolerance float64 // 容差范围
}

// 预定义的标准宽高比（只保留常用比例）
var standardAspectRatios = []AspectRatio{
	{Name: "1_1", Ratio: 1.0, Tolerance: aspectRatioTolerance},
	{Name: "4_3", Ratio: 4.0 / 3.0, Tolerance: aspectRatioTolerance},
	{Name: "16_9", Ratio: 16.0 / 9.0, Tolerance: aspectRatioTolerance},
	{Name: "3_2", Ratio: 3.0 / 2.0, Tolerance: aspectRatioTolerance},
}

// ImageAnalyzer 图片分析器
type ImageAnalyzer struct{}

// NewImageAnalyzer 创建图片分析器
func NewImageAnalyzer() *ImageAnalyzer {
	return &ImageAnalyzer{}
}

// ============================================================
// 图片尺寸获取相关方法
// ============================================================

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
		return 0, 0, fmt.Errorf("exiftool执行失败: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) != 2 {
		return 0, 0, fmt.Errorf("exiftool输出格式不正确，期望2行，实际%d行", len(lines))
	}

	return ia.parseDimensions(lines[0], lines[1])
}

// tryImageMagick 尝试使用ImageMagick的magick命令获取图片尺寸
func (ia *ImageAnalyzer) tryImageMagick(imagePath string) (width, height int, err error) {
	cmd := exec.Command("magick", "identify", "-format", "%w %h", imagePath)
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}

	parts := strings.Fields(strings.TrimSpace(string(output)))
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("magick identify输出格式不正确")
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
	cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "csv=p=0",
		"-select_streams", "v:0", "-show_entries", "stream=width,height", imagePath)
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("ffprobe执行失败: %v", err)
	}

	parts := strings.Split(strings.TrimSpace(string(output)), ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("ffprobe输出格式不正确，期望2个字段，实际%d个", len(parts))
	}

	return ia.parseDimensions(parts[0], parts[1])
}

// getDefaultDimensions 返回默认的图片尺寸，并返回合理的宽高比
func (ia *ImageAnalyzer) getDefaultDimensions() (width, height int) {
	return defaultImageWidth, defaultImageHeight
}

// parseDimensions 解析宽高字符串为整数（通用辅助方法）
func (ia *ImageAnalyzer) parseDimensions(widthStr, heightStr string) (width, height int, err error) {
	width, err = strconv.Atoi(strings.TrimSpace(widthStr))
	if err != nil {
		return 0, 0, fmt.Errorf("解析宽度失败: %v", err)
	}

	height, err = strconv.Atoi(strings.TrimSpace(heightStr))
	if err != nil {
		return 0, 0, fmt.Errorf("解析高度失败: %v", err)
	}

	return width, height, nil
}

// ============================================================
// 宽高比计算相关方法
// ============================================================

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

// ============================================================
// 图片格式转换相关方法
// ============================================================

// ConvertToJPG 将 .tmp 文件转换为 .jpg 格式
// 流程：
// 1. 使用 exiftool 检测真实格式
// 2. 使用 ImageMagick 转换为 .jpg
// 3. 删除 .tmp 文件
// 4. 返回 .jpg 文件路径
func (ia *ImageAnalyzer) ConvertToJPG(tmpPath string) (jpgPath string, err error) {
	// 1. 检测真实格式
	realFormat, err := ia.DetectImageFormat(tmpPath)
	if err != nil {
		logx.Infof("无法检测图片真实格式: %v，将继续尝试转换", err)
		realFormat = "UNKNOWN"
	}

	// 2. 生成 JPG 文件路径（去掉 .tmp 后缀，加上 .jpg）
	jpgPath = strings.TrimSuffix(tmpPath, ".tmp") + ".jpg"

	// 3. 使用 ImageMagick 转换
	logx.Infof("开始转换图片: %s -> JPG (检测到的格式: %s)", tmpPath, realFormat)
	err = ia.convertWithImageMagick(tmpPath, jpgPath)
	if err != nil {
		return "", fmt.Errorf("ImageMagick转换失败: %v", err)
	}

	// 4. 删除 .tmp 文件
	if removeErr := os.Remove(tmpPath); removeErr != nil {
		logx.Errorf("删除临时文件失败: %s, 错误: %v", tmpPath, removeErr)
		// 不影响主流程，继续执行
	}

	// 5. 对于某些格式（如 BMP），转换后可能缺少 EXIF 信息，需要补充
	if shouldWriteExif(realFormat) {
		logx.Infof("补充 EXIF 信息: %s (格式: %s)", jpgPath, realFormat)
		ia.writeBasicExifInfo(jpgPath) // 不影响主流程，失败也继续
	}

	logx.Infof("图片转换成功: %s -> %s (格式: %s)", tmpPath, jpgPath, realFormat)
	return jpgPath, nil
}

// DetectImageFormat 检测图片的真实格式（不依赖扩展名）
// 使用 exiftool 命令检测，支持本地文件
// 返回真实的文件格式（如 "HEIC", "JPEG", "PNG", "BMP", "WEBP" 等）
func (ia *ImageAnalyzer) DetectImageFormat(imagePath string) (format string, err error) {
	// 使用 exiftool -FileType 获取真实格式
	// -s3: 只输出值，不输出字段名
	// FileType 返回文件的真实类型，不受扩展名影响
	cmd := exec.Command("exiftool", "-FileType", "-s3", imagePath)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("无法识别图片格式: %v", err)
	}

	format = strings.TrimSpace(strings.ToUpper(string(output)))
	logx.Infof("检测到图片真实格式: %s (来源: %s)", format, imagePath)
	return format, nil
}

// convertWithImageMagick 使用 ImageMagick 的 magick 命令转换图片
// 支持的格式：heic, heif, webp, jfif, bmp, tiff 等
// 转换为 JPG 格式，质量设置为 100%（尽量保留原图质量）
// 转换时会移除原始 EXIF 数据，并设置新的时间戳
// srcPath 可以是本地文件路径，也可以是网络 URL
func (ia *ImageAnalyzer) convertWithImageMagick(srcPath, dstPath string) error {
	cmd := exec.Command("magick",
		srcPath,
		"-strip",                     // 移除所有原始 EXIF 数据
		"-set", "date:create", "now", // 设置创建时间为当前时间
		"-set", "date:modify", "now", // 设置修改时间为当前时间
		"-quality", jpegConversionQuality, // 设置 JPEG 质量
		dstPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("magick命令执行失败: %v, 输出: %s", err, string(output))
	}

	// 验证输出文件是否存在且不为空
	stat, err := os.Stat(dstPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("转换后的文件不存在: %s", dstPath)
	}
	if err != nil {
		return fmt.Errorf("验证转换结果失败: %v", err)
	}
	if stat.Size() == 0 {
		return fmt.Errorf("转换后的文件为空: %s", dstPath)
	}

	return nil
}

// shouldWriteExif 判断该格式转换后是否需要补充 EXIF 信息
func shouldWriteExif(format string) bool {
	format = strings.ToUpper(format)
	switch format {
	case "BMP", "BMP3", "DIB":
		// BMP 系列格式转换后通常缺少 EXIF 信息
		return true
	case "TIFF", "TIF":
		// TIFF 格式有时也需要补充
		return true
	default:
		// 其他格式通常不需要
		return false
	}
}

// writeBasicExifInfo 为转换后的图片写入基础 EXIF 信息
// 解决某些格式（如 BMP3）转换后缺少 ImageWidth、ImageHeight 等基础信息的问题
func (ia *ImageAnalyzer) writeBasicExifInfo(imagePath string) error {
	// 1. 先获取图片的实际尺寸
	width, height, err := ia.GetImageDimensions(imagePath)
	if err != nil {
		logx.Infof("无法获取图片尺寸，跳过写入 EXIF: %v", err)
		return nil // 不返回错误，因为这不是关键操作
	}

	// 2. 使用 exiftool 写入基础 EXIF 信息
	// -overwrite_original: 不创建备份文件
	// -ImageWidth, -ImageHeight: 设置图片尺寸
	// -Orientation=1: 设置正常方向
	cmd := exec.Command("exiftool",
		"-overwrite_original",
		fmt.Sprintf("-ImageWidth=%d", width),
		fmt.Sprintf("-ImageHeight=%d", height),
		"-Orientation=1",
		imagePath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logx.Infof("写入 EXIF 信息失败: %v, 输出: %s", err, string(output))
		return nil // 不返回错误，因为这不是关键操作
	}

	logx.Infof("成功写入基础 EXIF 信息: %s (尺寸: %dx%d)", imagePath, width, height)
	return nil
}
