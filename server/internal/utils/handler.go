package utils

import (
	"net/http"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 统一响应格式
type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// Success 成功响应
func Success(data interface{}) *Response {
	return &Response{
		Code: 200,
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
	} else {
		// 错误返回
		errcode := 500
		errmsg := "服务器开小差啦，稍后再来试一试"

		// 这里可以根据具体的错误类型进行处理
		// 目前先简单处理，后续可以扩展自定义错误类型
		logx.WithContext(r.Context()).Errorf("【API-ERR】 : %+v ", err)

		httpx.WriteJson(w, http.StatusBadRequest, Error(errcode, errmsg))
	}
}
