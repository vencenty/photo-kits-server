PHOTO_SERVER_DIR:= ./server
PHOTO_BIN_DIR:= $(PHOTO_SERVER_DIR)/cmd
PHOTO_JOB_DIR := $(PHOTO_SERVER_DIR)/job

gen-model:
	echo "开始生成数据库model..."
	goctl model mysql datasource --url  "root:UvbGrsVVaKDDzOEF@tcp(vencenty.cc:53824)/photo-kits"  -d ./server/model -t photo --style=goZero
	goctl model mysql datasource --url  "root:UvbGrsVVaKDDzOEF@tcp(vencenty.cc:53824)/photo-kits"  -d ./server/model -t order --style=goZero
	echo "数据库模型model生成结束"


gen-api:
	echo "开始生成api..."
	goctl api go -api ./server/api/server.api -dir ./server --style=goZero


build:
	mkdir -p $(PHOTO_BIN_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(PHOTO_BIN_DIR)/main-server $(PHOTO_SERVER_DIR)/photo.go
	GOOS=linux GOARCH=amd64 go build -o $(PHOTO_BIN_DIR)/photo-sync $(PHOTO_JOB_DIR)/photoSync/main.go

