# Gather - Ethereum Data Collector Makefile

# 变量定义
GO := go
BINARY_DIR := bin
COLLECTOR_BINARY := $(BINARY_DIR)/gather-collector
API_BINARY := $(BINARY_DIR)/gather-api
MAIN_COLLECTOR := cmd/gather/main.go
MAIN_API := cmd/api/main.go

# 版本信息
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 构建标志
LDFLAGS := -ldflags "-X 'main.Version=$(VERSION)' -X 'main.BuildTime=$(BUILD_TIME)' -X 'main.GitCommit=$(GIT_COMMIT)'"

# 默认目标
.DEFAULT_GOAL := build

# 帮助信息
.PHONY: help
help: ## 显示帮助信息
	@echo "Gather - Ethereum Data Collector"
	@echo ""
	@echo "可用命令:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# 创建二进制目录
$(BINARY_DIR):
	@mkdir -p $(BINARY_DIR)

# 构建所有二进制文件
.PHONY: build
build: $(COLLECTOR_BINARY) $(API_BINARY) ## 构建所有二进制文件

# 构建采集器
$(COLLECTOR_BINARY): $(BINARY_DIR)
	@echo "构建采集器..."
	$(GO) build $(LDFLAGS) -o $(COLLECTOR_BINARY) $(MAIN_COLLECTOR)
	@echo "采集器构建完成: $(COLLECTOR_BINARY)"

# 构建API服务器
$(API_BINARY): $(BINARY_DIR)
	@echo "构建API服务器..."
	$(GO) build $(LDFLAGS) -o $(API_BINARY) $(MAIN_API)
	@echo "API服务器构建完成: $(API_BINARY)"

# 构建采集器（单独）
.PHONY: build-collector
build-collector: $(COLLECTOR_BINARY) ## 只构建采集器

# 构建API服务器（单独）
.PHONY: build-api
build-api: $(API_BINARY) ## 只构建API服务器

# 交叉编译
.PHONY: build-linux
build-linux: $(BINARY_DIR) ## 构建Linux版本
	@echo "构建Linux版本..."
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_DIR)/gather-collector-linux-amd64 $(MAIN_COLLECTOR)
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_DIR)/gather-api-linux-amd64 $(MAIN_API)

.PHONY: build-windows
build-windows: $(BINARY_DIR) ## 构建Windows版本
	@echo "构建Windows版本..."
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_DIR)/gather-collector-windows-amd64.exe $(MAIN_COLLECTOR)
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_DIR)/gather-api-windows-amd64.exe $(MAIN_API)

.PHONY: build-darwin
build-darwin: $(BINARY_DIR) ## 构建macOS版本
	@echo "构建macOS版本..."
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_DIR)/gather-collector-darwin-amd64 $(MAIN_COLLECTOR)
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_DIR)/gather-api-darwin-amd64 $(MAIN_API)

.PHONY: build-all-platforms
build-all-platforms: build-linux build-windows build-darwin ## 构建所有平台版本

# 运行程序
.PHONY: run-collector
run-collector: build-collector ## 运行采集器
	@echo "启动采集器..."
	./$(COLLECTOR_BINARY) --config configs/config.yaml

.PHONY: run-api
run-api: build-api ## 运行API服务器
	@echo "启动API服务器..."
	./$(API_BINARY) --config configs/config.yaml --port 8080

# 测试
.PHONY: test
test: ## 运行所有测试
	@echo "运行测试..."
	$(GO) test -v ./...

.PHONY: test-coverage
test-coverage: ## 运行测试并生成覆盖率报告
	@echo "运行测试并生成覆盖率报告..."
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告已生成: coverage.html"

.PHONY: test-race
test-race: ## 运行竞态检测测试
	@echo "运行竞态检测测试..."
	$(GO) test -v -race ./...

.PHONY: benchmark
benchmark: ## 运行基准测试
	@echo "运行基准测试..."
	$(GO) test -v -bench=. -benchmem ./...

# 代码质量
.PHONY: fmt
fmt: ## 格式化代码
	@echo "格式化代码..."
	$(GO) fmt ./...

.PHONY: vet
vet: ## 运行go vet
	@echo "运行go vet..."
	$(GO) vet ./...

.PHONY: lint
lint: ## 运行golangci-lint（需要安装golangci-lint）
	@echo "运行golangci-lint..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint 未安装，跳过检查"; \
		echo "安装命令: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

.PHONY: check
check: fmt vet lint test ## 运行所有代码检查

# 依赖管理
.PHONY: mod-tidy
mod-tidy: ## 整理Go模块依赖
	@echo "整理Go模块依赖..."
	$(GO) mod tidy

.PHONY: mod-download
mod-download: ## 下载Go模块依赖
	@echo "下载Go模块依赖..."
	$(GO) mod download

.PHONY: mod-verify
mod-verify: ## 验证Go模块依赖
	@echo "验证Go模块依赖..."
	$(GO) mod verify

# 清理
.PHONY: clean
clean: ## 清理构建文件
	@echo "清理构建文件..."
	rm -rf $(BINARY_DIR)
	rm -f coverage.out coverage.html

.PHONY: clean-logs
clean-logs: ## 清理日志文件
	@echo "清理日志文件..."
	find . -name "*.log" -type f -delete
	rm -rf logs/*

.PHONY: clean-outputs
clean-outputs: ## 清理输出文件
	@echo "清理输出文件..."
	rm -rf outputs/*

.PHONY: clean-all
clean-all: clean clean-logs clean-outputs ## 清理所有生成的文件

# 安装
.PHONY: install
install: build ## 安装到$GOPATH/bin
	@echo "安装到 $$GOPATH/bin..."
	$(GO) install $(LDFLAGS) $(MAIN_COLLECTOR)
	$(GO) install $(LDFLAGS) $(MAIN_API)

# Docker
.PHONY: docker-build
docker-build: ## 构建Docker镜像
	@echo "构建Docker镜像..."
	docker build -t gather:$(VERSION) -t gather:latest .

.PHONY: docker-run
docker-run: ## 运行Docker容器
	@echo "运行Docker容器..."
	docker run -d --name gather-collector \
		-v $(PWD)/configs:/app/configs \
		-v $(PWD)/outputs:/app/outputs \
		-v $(PWD)/data:/app/data \
		gather:latest

.PHONY: docker-stop
docker-stop: ## 停止Docker容器
	@echo "停止Docker容器..."
	docker stop gather-collector || true
	docker rm gather-collector || true

# 配置
.PHONY: setup-config
setup-config: ## 设置配置文件
	@echo "设置配置文件..."
	@if [ ! -f configs/config.yaml ]; then \
		cp configs/config.example.yaml configs/config.yaml; \
		echo "已从示例创建配置文件: configs/config.yaml"; \
		echo "请编辑配置文件以适应您的环境"; \
	else \
		echo "配置文件已存在: configs/config.yaml"; \
	fi
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "已从示例创建环境变量文件: .env"; \
		echo "请编辑环境变量文件以设置您的密钥"; \
	else \
		echo "环境变量文件已存在: .env"; \
	fi

.PHONY: setup-dirs
setup-dirs: ## 创建必要的目录
	@echo "创建必要的目录..."
	mkdir -p outputs data logs configs

.PHONY: setup
setup: setup-dirs setup-config ## 初始化项目设置

# 版本信息
.PHONY: version
version: ## 显示版本信息
	@echo "版本: $(VERSION)"
	@echo "构建时间: $(BUILD_TIME)"
	@echo "Git提交: $(GIT_COMMIT)"

# 发布
.PHONY: release
release: clean build-all-platforms test ## 准备发布版本
	@echo "准备发布版本 $(VERSION)..."
	mkdir -p release
	tar -czf release/gather-$(VERSION)-linux-amd64.tar.gz -C $(BINARY_DIR) gather-collector-linux-amd64 gather-api-linux-amd64
	tar -czf release/gather-$(VERSION)-darwin-amd64.tar.gz -C $(BINARY_DIR) gather-collector-darwin-amd64 gather-api-darwin-amd64
	zip -j release/gather-$(VERSION)-windows-amd64.zip $(BINARY_DIR)/gather-collector-windows-amd64.exe $(BINARY_DIR)/gather-api-windows-amd64.exe
	@echo "发布文件已准备完成在 release/ 目录"

# 开发
.PHONY: dev
dev: setup build ## 开发环境设置
	@echo "开发环境已准备就绪"
	@echo "运行 'make run-collector' 启动采集器"
	@echo "运行 'make run-api' 启动API服务器"

# 监控（需要安装 air 工具进行热重载）
.PHONY: watch
watch: ## 监控文件变化并自动重新构建（需要安装air）
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "air 未安装，无法启用热重载"; \
		echo "安装命令: go install github.com/cosmtrek/air@latest"; \
		echo "然后创建 .air.toml 配置文件"; \
	fi

# 性能分析
.PHONY: pprof-collector
pprof-collector: ## 对采集器进行性能分析
	@echo "启动采集器性能分析..."
	$(GO) run $(MAIN_COLLECTOR) --config configs/config.yaml --pprof

.PHONY: pprof-api
pprof-api: ## 对API服务器进行性能分析
	@echo "启动API服务器性能分析..."
	$(GO) run $(MAIN_API) --config configs/config.yaml --port 8080 --pprof

# 文档
.PHONY: docs
docs: ## 生成文档
	@echo "生成文档..."
	@echo "API文档位于: docs/API.md"
	@echo "部署文档位于: docs/DEPLOYMENT.md"

# 特殊变量
.PHONY: list
list: ## 列出所有可用目标
	@$(MAKE) -pRrq -f $(lastword $(MAKEFILE_LIST)) : 2>/dev/null | awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | sort | egrep -v -e '^[^[:alnum:]]' -e '^$@$$'