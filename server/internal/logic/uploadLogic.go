package logic

import (
	"context"
	"fmt"
	"github.com/zeromicro/go-zero/core/jsonx"
	"net/http"
	"os"
	"path"
	"photo-kits-server/server/internal/svc"
	"photo-kits-server/server/internal/types"
	"photo-kits-server/server/model"

	"github.com/zeromicro/go-zero/core/logx"
)

type UploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	r      *http.Request
}

func NewUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext, r *http.Request) *UploadLogic {
	return &UploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
		r:      r,
	}
}

func (l *UploadLogic) Upload() (resp *types.UploadResponse, err error) {
	file, handler, err := l.r.FormFile("file")
	if err != nil {
		return nil, fmt.Errorf("upload file error: %v", err)
	}
	defer file.Close()
	logx.Infof("upload file: %+v, file size: %d, MIME header: %+v",
		handler.Filename, handler.Size, handler.Header)

	tempFile, err := os.Create(path.Join(l.svcCtx.Config.Path, handler.Filename))
	if err != nil {
		return nil, err
	}
	defer tempFile.Close()
	//io.Copy(tempFile, file)

	orderModel := model.NewOrdersModel(l.svcCtx.DB)
	sn, err := orderModel.FindOneByOrderSn(l.ctx, "42")
	fmt.Println(sn, err)
	r, err := jsonx.Marshal(sn)

	return &types.UploadResponse{
		Message: string(r),
	}, nil

}
