package admin

import (
	"context"
	"database/sql"
	"errors"
	"github.com/golang-jwt/jwt/v4"
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

func getJwtToken(secretKey string, iat int64, seconds int64, id int64) (string, error) {
	// 创建一个 MapClaims 类型的声明
	claims := make(jwt.MapClaims)
	// 计算过期时间
	claims["exp"] = iat + seconds // 设置 JWT 的过期时间（exp），通常需要一个 UNIX 时间戳
	claims["iat"] = iat           // 设置签发时间（iat）
	claims["id"] = id             // 自定义的负载（payload），可以设置为任何信息，例如用户名、用户ID等
	// 创建新的 JWT
	token := jwt.New(jwt.SigningMethodHS256) // 使用 HMAC SHA256 签名方法创建新的 JWT
	// 将声明分配给 JWT
	token.Claims = claims
	// 使用 secretKey 签名JWT，并返回生成的字符串和错误（如果有）
	return token.SignedString([]byte(secretKey))
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

	secret := l.svcCtx.Config.Auth.AccessSecret
	expire := l.svcCtx.Config.Auth.AccessExpire
	//生成jwt token
	token, err := getJwtToken(secret, time.Now().Unix(), expire, admin.Id)
	if err != nil {
		return nil, errors.New("生成数据错误")
	}

	resp = &types.AdminLoginResponse{
		Token: token,
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
