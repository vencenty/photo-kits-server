# 图片格式转换功能

## 功能概述

photoSync同步服务支持自动将特殊格式的照片转换为标准JPG格式。当检测到下载的照片是 **HEIC**、**HEIF**、**WebP** 或 **JFIF** 格式时，系统会自动调用ImageMagick进行格式转换，确保最终存储的照片都是标准的JPG格式，便于跨平台查看和使用。

该功能完全自动化，无需手动配置，转换失败时会优雅降级，不影响整体同步流程。

## 支持的格式

### 需要转换为JPG的格式
- `.heic` / `.heif` - Apple的HEIC/HEIF格式（iPhone常用）
- `.webp` - WebP格式（Google开发的现代图片格式）
- `.jfif` - JFIF格式（JPEG文件交换格式）

### 不需要转换的格式
- `.jpg` / `.jpeg` - 已经是标准JPG格式，直接保存
- 其他格式（如`.png`, `.gif`等）- 保持原格式不变

## 技术实现

### 核心方法

在`image_analyzer.go`中提供了统一的格式转换方法：

```go
// ConvertToJPG 将特殊格式转换为标准JPG格式
func (ia *ImageAnalyzer) ConvertToJPG(imagePath string) (newPath string, converted bool, err error)

// needsConversion 判断是否需要转换
func (ia *ImageAnalyzer) needsConversion(fileExt string) bool

// convertWithImageMagick 使用ImageMagick执行转换
func (ia *ImageAnalyzer) convertWithImageMagick(srcPath, dstPath string) error
```

### 转换策略

系统使用 **ImageMagick** 作为统一的转换工具：

- 使用 `convert` 命令进行格式转换
- 设置JPEG质量为 **95%**，保证高质量输出
- 支持所有常见图片格式（heic, heif, webp, jfif, png, gif等）
- 转换成功后自动验证输出文件是否存在

### 集成到同步流程

在`syncer.go`的下载流程中自动集成：

1. 下载照片到临时位置
2. **自动检测并转换特殊格式**（heic/heif/webp/jfif → jpg）
3. 分析图片尺寸（转换后的JPG文件）
4. 创建目标目录（按规格和宽高比分类）
5. 移动文件到最终位置

## 安装依赖

### ImageMagick（推荐）

**macOS:**
```bash
brew install imagemagick
```

**Ubuntu/Debian:**
```bash
sudo apt-get install imagemagick
```

**CentOS/RHEL:**
```bash
sudo yum install ImageMagick
```

验证安装：
```bash
convert -version
```

## 转换流程

```
下载文件到临时目录
    ↓
检测文件扩展名
    ↓
需要转换？（.heic/.heif/.webp/.jfif）
    ├─ 是 → 使用ImageMagick转换
    │        ↓
    │        转换成功？
    │        ├─ 是 → 删除原文件 → 使用新JPG文件 → 继续处理
    │        └─ 否 → 记录错误 → 使用原文件 → 继续处理
    │
    └─ 否 → 直接使用原文件 → 继续处理
         (.jpg/.jpeg/其他格式)
```

## 日志输出

### 成功转换
```
检测到特殊格式 .webp，开始转换为 JPG
图片格式转换成功: .webp -> JPG
成功解析图片尺寸，格式: jpeg, 尺寸: 4000x3000
下载成功 url=https://example.com/photo.webp, 进度=1/10, 副本=1/1
```

### HEIC/HEIF格式转换
```
检测到特殊格式 .heic，开始转换为 JPG
图片格式转换成功: .heic -> JPG
成功解析图片尺寸，格式: jpeg, 尺寸: 4032x3024
```

### 转换失败（仍继续处理）
```
检测到特殊格式 .webp，开始转换为 JPG
格式转换失败: exec: "convert": executable file not found in $PATH，将使用原文件继续处理
格式转换失败，将使用原文件: url=https://example.com/photo.webp, err=ImageMagick转换失败...
Go标准库解析图片失败: image: unknown format，尝试使用外部工具
```

## 错误处理

系统采用"容错继续"的策略，确保转换失败不影响整体同步：

- **转换失败不中断**：即使格式转换失败，也会使用原文件继续处理
- **详细错误日志**：记录转换失败的详细原因，便于排查问题
- **自动降级**：转换失败时，如果原文件是可识别格式，尝试直接处理
- **外部工具回退**：如果Go标准库无法解析，自动尝试ImageMagick/exiftool/ffprobe

## 性能考虑

1. **原地转换**：在临时目录中完成转换，避免额外的文件复制开销
2. **高质量输出**：JPEG质量设置为95%，在文件大小和图片质量间取得最佳平衡
3. **自动清理**：转换成功后立即删除原格式文件，节省磁盘空间
4. **失败不阻塞**：转换失败不影响其他照片的同步，保证系统整体可用性
5. **批量处理**：支持多副本下载时，只需分析一次图片尺寸，提高效率

## 代码优化

相比之前的实现，新版本做了以下优化：

1. **统一格式处理**：heic/heif/webp/jfif都通过同一个`ConvertToJPG`方法处理
2. **简化逻辑**：使用ImageMagick作为统一转换工具，支持所有格式
3. **清晰的职责分离**：
   - `needsConversion()` - 判断是否需要转换
   - `ConvertToJPG()` - 执行格式转换
   - `GetImageDimensions()` - 获取图片尺寸
4. **更好的容错**：转换失败时优雅降级，不影响业务流程

## 兼容性

- ✅ 与现有的照片同步流程完全兼容
- ✅ 不影响原有的文件组织结构（按规格和宽高比分类）
- ✅ 不影响文件命名规则（数字序号命名）
- ✅ 支持多副本下载（num字段）
- ✅ 向后兼容：不影响已有的JPG/PNG/GIF等文件处理
- ✅ 自动处理：无需修改配置，自动识别并转换

## 注意事项

1. **必须安装ImageMagick**：这是唯一的依赖工具，但支持所有格式转换
   ```bash
   # 验证安装
   convert -version
   ```

2. **转换质量调整**：如需调整JPEG质量（当前95%），修改以下代码：
   ```go
   // image_analyzer.go 第270行
   cmd := exec.Command("convert", srcPath, "-quality", "95", dstPath)
   ```

3. **扩展支持格式**：如需支持更多格式转换，在`needsConversion()`方法中添加：
   ```go
   case ".png", ".gif":  // 添加新的格式
       return true
   ```

4. **HEIC格式支持**：现在统一通过ImageMagick转换，无需特殊处理
