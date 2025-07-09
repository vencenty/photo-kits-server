package utils

import (
	"net/http"

	xhttp "github.com/zeromicro/x/http"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// HandlerFunc 定义通用的handler函数类型
type HandlerFunc func(w http.ResponseWriter, r *http.Request) (interface{}, error)

// RequestParser 定义请求解析接口
type RequestParser interface {
	Parse(r *http.Request) error
}

// WrapHandler 包装handler，统一使用xhttp.JsonBaseResponseCtx返回响应
func WrapHandler(handler HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := handler(w, r)
		if err != nil {
			xhttp.JsonBaseResponseCtx(r.Context(), w, err)
		} else {
			xhttp.JsonBaseResponseCtx(r.Context(), w, resp)
		}
	}
}

// WrapHandlerWithRequest 包装带请求参数的handler
func WrapHandlerWithRequest(handler func(w http.ResponseWriter, r *http.Request, req interface{}) (interface{}, error), reqType interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := httpx.Parse(r, reqType); err != nil {
			xhttp.JsonBaseResponseCtx(r.Context(), w, err)
			return
		}

		resp, err := handler(w, r, reqType)
		if err != nil {
			xhttp.JsonBaseResponseCtx(r.Context(), w, err)
		} else {
			xhttp.JsonBaseResponseCtx(r.Context(), w, resp)
		}
	}
} 