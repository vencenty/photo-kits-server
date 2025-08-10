package main

import (
    "bufio"
    "errors"
    "flag"
    "fmt"
    "log"
    "math"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "strings"
)

type watermarkOptions struct {
    sourceDir              string
    outputDir              string
    color                  string
    marginPercent          float64
    fontSizePercentOfShort float64
}

func main() {
    opts := parseFlags()

    if err := validateEnvironment(); err != nil {
        log.Fatalf("依赖检查失败: %v", err)
    }

    total, processed, skipped, failed := 0, 0, 0, 0

    err := filepath.WalkDir(opts.sourceDir, func(path string, d os.DirEntry, walkErr error) error {
        if walkErr != nil {
            log.Printf("遍历错误: %v", walkErr)
            return nil
        }
        if d.IsDir() {
            return nil
        }
        if !isImageFile(path) {
            return nil
        }

        total++

        dateStr, hasDate, err := readShootDate(path)
        if err != nil {
            log.Printf("读取拍摄时间失败，跳过: file=%s, err=%v", path, err)
            skipped++
            return nil
        }
        if !hasDate {
            log.Printf("无拍摄时间，跳过: file=%s", path)
            skipped++
            return nil
        }

        width, height, err := readImageSize(path)
        if err != nil {
            log.Printf("获取尺寸失败，跳过: file=%s, err=%v", path, err)
            skipped++
            return nil
        }

        shortSide := float64(minInt(width, height))
        pointSize := int(shortSide * (opts.fontSizePercentOfShort / 100.0))
        if pointSize < 8 {
            pointSize = 8
        }

        marginX := int(float64(width) * (opts.marginPercent / 100.0))
        marginY := int(float64(height) * (opts.marginPercent / 100.0))

        rel, _ := filepath.Rel(opts.sourceDir, path)
        outPath := filepath.Join(opts.outputDir, rel)
        if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
            log.Printf("创建输出目录失败: dir=%s, err=%v", filepath.Dir(outPath), err)
            failed++
            return nil
        }

        if err := drawDateWatermark(path, outPath, dateStr, opts.color, pointSize, marginX, marginY); err != nil {
            log.Printf("加水印失败: file=%s, err=%v", path, err)
            failed++
            return nil
        }

        processed++
        log.Printf("加水印成功: %s -> %s, 日期=%s", path, outPath, dateStr)
        return nil
    })

    if err != nil {
        log.Fatalf("处理过程中发生错误: %v", err)
    }

    log.Printf("完成。总计=%d, 成功=%d, 跳过(无日期/尺寸失败)=%d, 失败=%d", total, processed, skipped, failed)
}

func parseFlags() watermarkOptions {
    var opts watermarkOptions
    flag.StringVar(&opts.sourceDir, "src", "", "源目录（必填）")
    flag.StringVar(&opts.outputDir, "out", "output", "输出目录（默认: output）")
    flag.StringVar(&opts.color, "color", "rgba(255,77,79,0.92)", "水印颜色（默认: 柔和暖红 rgba(255,77,79,0.92)）")
    flag.Float64Var(&opts.marginPercent, "margin", 5.0, "右下角边距百分比（默认: 5%）")
    flag.Float64Var(&opts.fontSizePercentOfShort, "fontPercent", 3.5, "字体大小占短边百分比（默认: 3.5%）")
    flag.Parse()

    if opts.sourceDir == "" {
        log.Fatalf("请通过 -src 指定源目录")
    }
    return opts
}

func validateEnvironment() error {
    if _, err := exec.LookPath("exiftool"); err != nil {
        return errors.New("未找到 exiftool，可通过 brew install exiftool 安装")
    }
    if _, err := exec.LookPath("identify"); err != nil {
        return errors.New("未找到 ImageMagick 的 identify，可通过 brew install imagemagick 安装")
    }
    if _, err := exec.LookPath("convert"); err != nil {
        return errors.New("未找到 ImageMagick 的 convert，可通过 brew install imagemagick 安装")
    }
    return nil
}

func isImageFile(path string) bool {
    ext := strings.ToLower(filepath.Ext(path))
    switch ext {
    case ".jpg", ".jpeg", ".png", ".webp", ".tif", ".tiff", ".gif", ".bmp", ".heic", ".heif":
        return true
    default:
        return false
    }
}

func readShootDate(path string) (string, bool, error) {
    // 优先 DateTimeOriginal，其次 CreateDate/ModifyDate
    // exiftool -s -s -s -DateTimeOriginal -CreateDate -ModifyDate file
    cmd := exec.Command("exiftool", "-s", "-s", "-s", "-DateTimeOriginal", "-CreateDate", "-ModifyDate", path)
    output, err := cmd.Output()
    if err != nil {
        return "", false, err
    }
    scanner := bufio.NewScanner(strings.NewReader(string(output)))
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" {
            continue
        }
        // 期望格式: 2006:01:02 15:04:05 或变体
        date := extractYMD(line)
        if date != "" {
            return date, true, nil
        }
    }
    return "", false, nil
}

func extractYMD(exifDate string) string {
    // 常见格式: YYYY:MM:DD HH:MM:SS, 或 YYYY-MM-DDTHH:MM:SSZ 等
    s := strings.TrimSpace(exifDate)
    if len(s) < 10 {
        return ""
    }
    // 替换分隔符，统一提取前10位
    s = strings.ReplaceAll(s, "-", ":")
    s = strings.ReplaceAll(s, "/", ":")
    ymd := s[:10]
    parts := strings.Split(ymd, ":")
    if len(parts) != 3 {
        return ""
    }
    year, month, day := parts[0], parts[1], parts[2]
    if len(year) != 4 || len(month) != 2 || len(day) != 2 {
        return ""
    }
    return fmt.Sprintf("%s/%s/%s", year, month, day)
}

func readImageSize(path string) (int, int, error) {
    // identify -format "%w %h" file
    cmd := exec.Command("identify", "-format", "%w %h", path)
    out, err := cmd.Output()
    if err != nil {
        return 0, 0, err
    }
    fields := strings.Fields(strings.TrimSpace(string(out)))
    if len(fields) != 2 {
        return 0, 0, fmt.Errorf("identify 输出异常: %s", string(out))
    }
    w, err := strconv.Atoi(fields[0])
    if err != nil {
        return 0, 0, err
    }
    h, err := strconv.Atoi(fields[1])
    if err != nil {
        return 0, 0, err
    }
    return w, h, nil
}

func drawDateWatermark(inPath, outPath, date, color string, pointSize, marginX, marginY int) error {
    // convert in -gravity southeast -fill color -pointsize N -annotate +x+y "date" out
    // 采用轻度抗锯齿，避免失真
    args := []string{
        inPath,
        "-gravity", "southeast",
        "-fill", color,
        "-pointsize", strconv.Itoa(pointSize),
        "-annotate", fmt.Sprintf("+%d+%d", maxInt(marginX, 0), maxInt(marginY, 0)),
        date,
        outPath,
    }
    cmd := exec.Command("convert", args...)
    if out, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("convert 失败: %v, 输出: %s", err, string(out))
    }
    return nil
}

func minInt(a, b int) int {
    if a < b {
        return a
    }
    return b
}

func maxInt(a, b int) int {
    if a > b {
        return a
    }
    return b
}

// avoid unused import of math in future changes
var _ = math.Min


