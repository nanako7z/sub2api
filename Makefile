.PHONY: build build-backend build-frontend build-datamanagementd test test-backend test-frontend test-datamanagementd secret-scan mitm-compare

# 一键编译前后端
build: build-backend build-frontend

# 编译后端（复用 backend/Makefile）
build-backend:
	@$(MAKE) -C backend build

# 编译前端（需要已安装依赖）
build-frontend:
	@pnpm --dir frontend run build

# 编译 datamanagementd（宿主机数据管理进程）
build-datamanagementd:
	@cd datamanagement && go build -o datamanagementd ./cmd/datamanagementd

# 运行测试（后端 + 前端）
test: test-backend test-frontend

test-backend:
	@$(MAKE) -C backend test

test-frontend:
	@pnpm --dir frontend run lint:check
	@pnpm --dir frontend run typecheck

test-datamanagementd:
	@cd datamanagement && go test ./...

secret-scan:
	@python3 tools/secret_scan.py

# MITM 抓包对比回归（用法: make mitm-compare BASELINE=... CANDIDATE=...）
mitm-compare:
	@test -n "$(BASELINE)" || (echo "BASELINE is required"; exit 1)
	@test -n "$(CANDIDATE)" || (echo "CANDIDATE is required"; exit 1)
	@python3 tools/mitm/compare_captures.py --strict --baseline "$(BASELINE)" --candidate "$(CANDIDATE)"
