package photo

import (
	"context"
	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"photo-kits-server/server/internal/svc"
	"photo-kits-server/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SubmitLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	db     sqlx.SqlConn
}

func NewSubmitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubmitLogic {
	db := sqlx.NewSqlConn("mysql", svcCtx.Config.Database.DataSource)
	return &SubmitLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
		db:     db,
	}
}

func (l *SubmitLogic) Submit(req *types.SubmitRequest) error {

	return nil
}
