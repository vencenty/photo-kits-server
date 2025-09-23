package utils

import (
	"net/http"
	"reflect"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 统一响应格式
type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// 错误码接口，用于提取自定义错误的错误码
type ErrorCode interface {
	GetCode() int
	GetMessage() string
}

// Success 成功响应
func Success(data interface{}) *Response {
	return &Response{
		Code: 0,
		Msg:  "success",
		Data: data,
	}
}

// Error 错误响应
func Error(code int, msg string) *Response {
	return &Response{
		Code: code,
		Msg:  msg,
	}
}

// HttpResult 统一的HTTP响应处理函数
func HttpResult(r *http.Request, w http.ResponseWriter, resp interface{}, err error) {
	if err == nil {
		// 成功返回
		result := Success(resp)
		httpx.WriteJson(w, http.StatusOK, result)
		return
	}

	// 错误返回 - 区分业务错误和系统错误
	code := 500
	msg := "服务器开小差啦，稍后再来试一试"

	// 尝试类型断言，检查是否实现了 ErrorCode 接口
	if errorCode, ok := err.(ErrorCode); ok {
		// 自定义错误类型，直接使用其错误码和消息
		code = errorCode.GetCode()
		msg = errorCode.GetMessage()
		logx.WithContext(r.Context()).Infof("【BUSINESS-ERR】 code: %d, msg: %s", code, msg)
	} else {
		// 尝试通过反射获取字段 - 处理 zeromicro/x/errors 包的错误类型
		if errValue := reflect.ValueOf(err); errValue.IsValid() {
			if errValue.Kind() == reflect.Ptr {
				errValue = errValue.Elem()
			}

			if errValue.IsValid() {
				// 尝试获取 Code 字段
				if codeField := errValue.FieldByName("Code"); codeField.IsValid() && codeField.CanInterface() {
					if codeInt, ok := codeField.Interface().(int); ok {
						code = codeInt

						// 尝试获取 Message 字段
						if msgField := errValue.FieldByName("Msg"); msgField.IsValid() && msgField.CanInterface() {
							if msgStr, ok := msgField.Interface().(string); ok {
								msg = msgStr
							}
						}

						// 如果成功获取到自定义错误码，说明是业务错误
						logx.WithContext(r.Context()).Infof("【BUSINESS-ERR】 code: %d, msg: %s", code, msg)
					}
				} else {
					// 没有 Code 字段，说明是系统错误
					logx.WithContext(r.Context()).Errorf("【SYSTEM-ERR】 : %+v", err)
				}
			}
		} else {
			// 无法解析的错误，当作系统错误处理
			logx.WithContext(r.Context()).Errorf("【SYSTEM-ERR】 : %+v", err)
		}
	}

	// 使用统一的错误响应格式，HTTP状态码统一返回200
	result := Error(code, msg)
	httpx.WriteJson(w, http.StatusOK, result)
}
