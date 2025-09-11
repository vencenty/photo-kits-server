package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"
	"github.com/zeromicro/go-zero/core/logx"
	"io"
	"net/http"
	"os"
	"os/exec"

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
	// todo: add your logic here and delete this line

	file, handler, err := l.request.FormFile("file")
	if err != nil {
		return
	}
	_ = handler

	// 把上传的文件读入内存
	//data, err := io.ReadAll(file)
	//if err != nil {
	//	return
	//}

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

	// 调用 exiftool 获取所有元数据
	cmd := exec.Command("exiftool", "-j", tmpFile.Name()) // -j 输出 JSON
	output, err := cmd.Output()
	//fmt.Println("outputResult=", output)
	if err != nil {
		logx.Error(err)
		return
	}

	resp = &types.MetaResponse{}

	var result = []map[string]interface{}{}
	_ = json.Unmarshal(output, &result)

	if len(result) > 0 {
		resp.Data = result[0]
	} else {
		resp.Data = map[string]interface{}{}
	}

	// 输出 JSON
	return
}

//Make/Model/Lens/ExposureTime/FNumber/ISO/DateTimeOriginal/Orientation/Flash

// walker 实现 exif.Walker 接口
type walker struct{}

func (w walker) Walk(name exif.FieldName, tag *tiff.Tag) error {
	fmt.Printf("%s: %s\n", name, tag.String())
	return nil
}
