#!/usr/bin/env sh

echo "开始生成数据库model..."
goctl model mysql datasource --url  "root:UvbGrsVVaKDDzOEF@tcp(vencenty.cc:53824)/photo-kits"  -d ./server/model -t photo --style=goZero
goctl model mysql datasource --url  "root:UvbGrsVVaKDDzOEF@tcp(vencenty.cc:53824)/photo-kits"  -d ./server/model -t order --style=goZero
echo "数据库模型model生成结束"

echo "开始生成api..."
goctl api go -api ./server/api/server.api -dir ./server --style=goZero

