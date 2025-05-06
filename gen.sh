#!/usr/bin/env sh

echo "开始生成api..."
goctl api go -api ./server/api/server -dir ./server --style=goZero
