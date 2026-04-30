#
# PROTO_FILES: gather all api/*.proto files in a cross-platform way.
#
# Goals:
# - Works on Linux/macOS (sh + find)
# - Works on Windows Git Bash / MSYS2 (sh + /usr/bin/find or git)
# - Works on Windows cmd.exe (fallback to for/dir)
#
# Strategy:
# - Prefer `git ls-files` (fast, consistent, no shell differences)
# - Fallback to find (POSIX shells)
# - Fallback to Windows cmd for-loop (cmd.exe only)
#
ifeq ($(OS),Windows_NT)
  # Windows: output proto paths relative to ./api/proto (and normalize to forward slashes).
  PROTO_FILES := $(shell powershell -NoProfile -Command "$$root=(Resolve-Path 'api/proto').Path.TrimEnd('\'); Get-ChildItem -Recurse -Filter *.proto 'api/proto' | ForEach-Object { ($$_.FullName.Substring($$root.Length+1)) -replace '\\\\','/' -replace '\\\\','/' }")
  PROTO_ROOT := $(shell cd)
else
  # Linux/macOS: use find (paths relative to ./api/proto)
  PROTO_FILES := $(shell find api/proto -name "*.proto" | sed 's#^api/proto/##')
  PROTO_ROOT := $(shell pwd)
endif

.PHONY: install
install:
	go install github.com/google/wire/cmd/wire@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	go install github.com/swaggo/swag/cmd/swag@latest

.PHONY: conf
conf:
	cp config.example.yaml config.yaml

.PHONY: generate
generate:
	go generate ./...

lint:
	golangci-lint run --config ./.golangci.yml

test:
	go test -v ./...

.PHONY: run
run:
	@go run ./cmd/lumenim $(filter-out run,$(MAKECMDGOALS))

.PHONY: dev-http dev-comet dev-queue dev-crontab
dev-http:
	@go run ./cmd/lumenim http
dev-comet:
	@go run ./cmd/lumenim comet
dev-queue:
	@go run ./cmd/lumenim queue
dev-crontab:
	@go run ./cmd/lumenim crontab

.PHONY: dev
dev:
	@$(MAKE) -j 4 dev-http dev-comet dev-queue dev-crontab

.PHONY: build
build:
	GOOS=linux GOARCH=amd64 go build -o ./bin/lumenim ./cmd/lumenim

.PHONY: build-stress-test
build-stress-test:
	cd cmd/stress-test && GOOS=linux GOARCH=amd64 go build -o stress-test
.PHONY: stress-test
stress-test: build-stress-test
	@echo "Stress test tool built successfully at cmd/stress-test/stress-test"
	@echo "Run './cmd/stress-test/stress-test --help' for usage"

.PHONY: protoc-gen-bff
protoc-gen-bff:
	go build -o protoc-gen-bff ./cmd/protoc-gen-bff

.PHONY: proto
proto: protoc-gen-bff
ifeq ($(strip $(PROTO_FILES)),)
	@echo "No .proto files found under ./api"
else
	@cd api/proto && \
	protoc \
		--plugin=protoc-gen-bff=../../protoc-gen-bff \
		--proto_path=. \
		--proto_path=../../third_party \
		--go_out=paths=source_relative:../../api/pb/ \
		--bff_out=../../api/pb/ \
		$(PROTO_FILES)
	@echo "protoc generate success"
endif
	@$(MAKE) proto-openapi
ifeq ($(OS),Windows_NT)
	@del /f /q protoc-gen-bff.exe 2>nul || exit 0
else
	@rm -f ./protoc-gen-bff 2>/dev/null || true
endif

.PHONY: proto-openapi # 生成 OpenApi 文档
proto-openapi:
ifeq ($(OS),Windows_NT)
	@echo "Skipping proto-openapi on Windows. Use Linux/WSL/Git Bash bash-compatible environment to generate OpenAPI."
else
	@for dir in $$(find api/proto -type d -mindepth 1 -maxdepth 1); do \
		echo "Processing directory: $$dir"; \
		proto_files=$$(find $$dir -name "*.proto" ! -name "article*"); \
		if [ -n "$$proto_files" ]; then \
		  protoc \
          			--proto_path=./api/proto \
          			--proto_path=./third_party \
          			--openapi_out=version=3:./$$dir \
          			$$proto_files; \
		fi; \
		echo "Generated OpenAPI spec for directory: $$dir";\
	done
endif

SWAG_BIN := $(shell go env GOPATH)/bin/swag

.PHONY: swagger-install # 安装 Swag 工具
swagger-install:
	go install github.com/swaggo/swag/cmd/swag@latest

.PHONY: swagger-gen # 生成 Swagger 文档
swagger-gen:
	$(SWAG_BIN) init -g internal/apis/server.go -o docs --parseDependency --parseInternal --useStructName --exclude internal/apis/handler/web/v1/article

.PHONY: swagger-fmt # 格式化 Swagger 注释
swagger-fmt:
	$(SWAG_BIN) fmt -g internal/apis/server.go


## 自定义命令
-include custom.mk