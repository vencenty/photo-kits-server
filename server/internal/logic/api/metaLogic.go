package api

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"
	"github.com/zeromicro/go-zero/core/logx"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"

	"server/internal/svc"
	"server/internal/types"
)

type MetaLogic struct {
	logx.Logger
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	request *http.Request
}

func NewMetaLogic(ctx context.Context, svcCtx *svc.ServiceContext, r *http.Request, w http.ResponseWriter) *MetaLogic {
	return &MetaLogic{
		Logger:  logx.WithContext(ctx),
		ctx:     ctx,
		svcCtx:  svcCtx,
		request: r,
	}
}

func (l *MetaLogic) Meta() (resp *types.MetaResponse, err error) {
	file, handler, err := l.request.FormFile("file")
	if err != nil {
		return
	}
	defer file.Close()

	// 获取文件扩展名
	_ = getFileExtension(handler.Filename)

	// 临时保存上传文件
	tmpFile, err := os.CreateTemp("", "upload-*"+handler.Filename)
	if err != nil {
		logx.Error(err)
		return
	}
	defer os.Remove(tmpFile.Name()) // 处理完删除
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, file)
	if err != nil {
		logx.Error(err)
		return
	}

	// 创建转换后的JPG文件路径
	convertedFile, err := os.CreateTemp("", "converted-*.jpg")
	if err != nil {
		logx.Error(err)
		return
	}
	defer os.Remove(convertedFile.Name()) // 处理完删除
	defer convertedFile.Close()

	// 使用ImageMagick转换图片为100%质量JPG，保留EXIF信息
	err = l.convertToJPG(tmpFile.Name(), convertedFile.Name())
	if err != nil {
		logx.Error("转换图片失败:", err)
		return
	}

	// 调用 exiftool 获取转换后图片的元数据
	cmd := exec.Command("exiftool", "-j", convertedFile.Name()) // -j 输出 JSON
	output, err := cmd.Output()
	if err != nil {
		logx.Error(err)
		return
	}

	// 计算转换后文件的SHA1以便命名
	sha1sum, err := computeFileSHA1(convertedFile.Name())
	if err != nil {
		logx.Error(err)
		return
	}

	// 初始化阿里云OSS客户端
	client, err := oss.New(l.svcCtx.Config.AliyunOSS.Endpoint,
		l.svcCtx.Config.AliyunOSS.AccessKeyId,
		l.svcCtx.Config.AliyunOSS.AccessKeySecret)
	if err != nil {
		logx.Errorf("ossClientInitError:%v", err)
		return
	}

	// 获取存储空间
	bucket, err := client.Bucket(l.svcCtx.Config.AliyunOSS.BucketName)
	if err != nil {
		logx.Errorf("ossBucketError:%v", err)
		return
	}

	// 生成对象名：convertImages/原名_sha1.jpg
	//originalNameNoExt := strings.TrimSuffix(handler.Filename, filepath.Ext(handler.Filename))
	ossFileName := fmt.Sprintf("%s.jpg", sha1sum)
	objectName := fmt.Sprintf("convertImages/%s", ossFileName)

	// 上传文件到OSS（本地文件路径上传）
	err = bucket.PutObjectFromFile(objectName, convertedFile.Name(), oss.ContentType("image/jpeg"))
	if err != nil {
		logx.Errorf("ossPutObjectError:%v", err)
		return
	}

	// 构建访问URL，使用代理域名
	fileURL := fmt.Sprintf("%s://%s/%s", l.svcCtx.Config.AliyunOSS.Schema, l.svcCtx.Config.AliyunOSS.ProxyDomain, objectName)

	resp = &types.MetaResponse{Url: fileURL}

	var result = []map[string]interface{}{}
	_ = json.Unmarshal(output, &result)

	if len(result) > 0 {
		resp.Data = result[0]
		resp.Url = fileURL
	}

	return
}

//Make/Model/Lens/ExposureTime/FNumber/ISO/DateTimeOriginal/Orientation/Flash

// getFileExtension 获取文件扩展名
func getFileExtension(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return "unknown"
	}
	return strings.ToLower(ext[1:]) // 去掉点号并转为小写
}

// convertToJPG 使用ImageMagick将图片转换为100%质量JPG，保留EXIF信息
func (l *MetaLogic) convertToJPG(inputPath, outputPath string) error {
	// 使用magick命令进行转换
	// -quality 100: 设置质量为100%
	// -strip: 不保留EXIF信息（我们需要保留，所以不加这个参数）
	// -auto-orient: 自动调整方向
	cmd := exec.Command("magick", inputPath, "-quality", "100", "-auto-orient", outputPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logx.Errorf("ImageMagick转换失败: %v, 输出: %s", err, string(output))
		return fmt.Errorf("图片转换失败: %v", err)
	}

	logx.Infof("图片转换成功: %s -> %s", inputPath, outputPath)
	return nil
}

// walker 实现 exif.Walker 接口
type walker struct{}

func (w walker) Walk(name exif.FieldName, tag *tiff.Tag) error {
	fmt.Printf("%s: %s\n", name, tag.String())
	return nil
}

// computeFileSHA1 计算文件的SHA1摘要
func computeFileSHA1(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
