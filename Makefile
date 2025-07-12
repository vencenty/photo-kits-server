#!/bin/sh
PHOTO_SERVER_DIR := ./server
PHOTO_BIN_DIR := $(PHOTO_SERVER_DIR)/cmd
PHOTO_JOB_DIR := $(PHOTO_SERVER_DIR)/job
GO_BUILD_FLAGS := -ldflags="-s -w" -tags no_k8s

gen-model:
	@echo "开始生成数据库 model..."
	cd $(PHOTO_SERVER_DIR) && \
	goctl model mysql datasource --url "root:UvbGrsVVaKDDzOEF@tcp(vencenty.cc:53824)/photo-kits" -d ./model -t photo --style=goZero && \
	goctl model mysql datasource --url "root:UvbGrsVVaKDDzOEF@tcp(vencenty.cc:53824)/photo-kits" -d ./model -t order --style=goZero && \
	echo "数据库模型 model 生成结束"

gen-api:
	@echo "开始生成 API..."
	cd $(PHOTO_SERVER_DIR) && \
	goctl api go -api ./api/server.api -dir . --style=goZero

build:
	@echo "开始构建..."
	cd $(PHOTO_SERVER_DIR) && \
	go mod tidy && \
	mkdir -p cmd && \
	GOOS=linux GOARCH=amd64 go build $(GO_BUILD_FLAGS) -o cmd/main-server photo.go && \
	GOOS=linux GOARCH=amd64 go build $(GO_BUILD_FLAGS) -o cmd/photo-sync job/photoSync/main.go
	@echo "构建完成，请在 $(PHOTO_BIN_DIR) 目录下查看构建结果"

sync-photos:
	$(PHOTO_BIN_DIR)/photo-sync -f $(PHOTO_SERVER_DIR)/etc/photo-api.yaml

run-sync:
	$(PHOTO_BIN_DIR)/photo-sync -f $(PHOTO_SERVER_DIR)/etc/photo-api.yaml > $(PHOTO_BIN_DIR)/photo-sync.log 2>&1 &

run-server:
	$(PHOTO_BIN_DIR)/main-server -f $(PHOTO_SERVER_DIR)/etc/photo-api.yaml > $(PHOTO_BIN_DIR)/main-server.log 2>&1 &
