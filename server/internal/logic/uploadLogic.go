package logic

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"photo-kits-server/server/internal/svc"
	"photo-kits-server/server/internal/types"

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
	io.Copy(tempFile, file)

	return &types.UploadResponse{
		Message: "",
	}, nil

}
