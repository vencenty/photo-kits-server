package photo

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"
	"photo-kits-server/server/internal/svc"
)

type PingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PingLogic {
	return &PingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PingLogic) Ping() (resp string, err error) {
	resp = "pong"
	return
}
