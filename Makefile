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
	GOOS=linux GOARCH=amd64 go build -o $(PHOTO_BIN_DIR)/main-server $(PHOTO_SERVER_DIR)/photo.go
	GOOS=linux GOARCH=amd64 go build -o $(PHOTO_BIN_DIR)/photo-sync $(PHOTO_JOB_DIR)/photoSync/main.go
	
	# 创建配置文件目录
	mkdir -p $(PHOTO_BIN_DIR)/etc
	mkdir -p $(PHOTO_BIN_DIR)/job/photoSync/etc
	
	# 复制配置文件
	cp $(PHOTO_SERVER_DIR)/etc/photo-api.yaml $(PHOTO_BIN_DIR)/etc/
	cp $(PHOTO_JOB_DIR)/photoSync/etc/syncConfig.yaml $(PHOTO_BIN_DIR)/job/photoSync/etc/
	cp $(PHOTO_JOB_DIR)/photoSync/etc/photoSync.yaml $(PHOTO_BIN_DIR)/job/photoSync/etc/
	
	echo "构建完成，请在 $(PHOTO_BIN_DIR) 目录下查看构建结果"
	
# 打包所有文件为tar.gz，方便部署
package: build
	tar -czvf $(PHOTO_BIN_DIR)/photo-kits.tar.gz -C $(PHOTO_BIN_DIR) main-server photo-sync etc job

