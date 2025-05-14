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
	echo "开始构建..."
	mkdir -p $(PHOTO_BIN_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -tags no_k8s -o $(PHOTO_BIN_DIR)/main-server $(PHOTO_SERVER_DIR)/photo.go
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -tags no_k8s -o $(PHOTO_BIN_DIR)/photo-sync $(PHOTO_JOB_DIR)/photoSync/main.go

	echo "构建完成，请在 $(PHOTO_BIN_DIR) 目录下查看构建结果"


run-sync:
	$(PHOTO_BIN_DIR)/photo-sync -f $(PHOTO_SERVER_DIR)/etc/photo-api.yaml > $(PHOTO_BIN_DIR)/photo-sync.log 2>&1 &

run-server:
	$(PHOTO_BIN_DIR)/main-server -f $(PHOTO_SERVER_DIR)/etc/photo-api.yaml > $(PHOTO_BIN_DIR)/main-server.log 2>&1 &
