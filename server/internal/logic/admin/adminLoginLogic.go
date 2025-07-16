package admin

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"server/internal/svc"
	"server/internal/types"
	"server/model"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
)

type AdminLoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminLoginLogic {
	return &AdminLoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminLoginLogic) AdminLogin(req *types.AdminLoginRequest) (resp *types.AdminLoginResponse, err error) {
	// 参数验证
	if req.Account == "" || req.Password == "" {
		return nil, errors.New("账号和密码不能为空")
	}

	// 根据账号查找管理员
	admin, err := l.svcCtx.AdminModel.FindByAccount(l.ctx, req.Account)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errors.New("账号不存在")
		}
		logx.Errorf("查找管理员失败: %v", err)
		return nil, errors.New("登录失败，请稍后重试")
	}

	// 验证密码
	if admin.PasswordHash.String == "" {
		return nil, errors.New("密码未设置")
	}

	err = bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash.String), []byte(req.Password))
	if err != nil {
		return nil, errors.New("密码错误")
	}

	// 构建响应
	resp = &types.AdminLoginResponse{
		Id:        admin.Id,
		Nickname:  admin.Nickname.String,
		AvatarUrl: admin.AvatarUrl.String,
		Email:     admin.Email.String,
		Mobile:    admin.Mobile.String,
		CreatedAt: formatTime(admin.CreatedAt),
		UpdatedAt: formatTime(admin.UpdatedAt),
	}

	return resp, nil
}

// formatTime 格式化时间为字符串
func formatTime(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format(time.RFC3339)
}
