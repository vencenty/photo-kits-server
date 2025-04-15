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

	return &types.UploadResponse{
		Message: string(r),
	}, nil

}
