PROJECT := aurga-web
INHERITED_TMPDIR := $(TMPDIR)
CI_ACTIVE := $(filter-out 0 false,$(strip $(CI)))
ifeq ($(strip $(WORKSPACE_TMP_BASE)),)
  ifneq ($(CI_ACTIVE),)
    TMP_BASE := $(or $(RUNNER_TEMP),$(INHERITED_TMPDIR),/tmp)
  else
    TMP_BASE := $(shell test -d /workspace/tmp -a -w /workspace/tmp && echo /workspace/tmp || echo $(or $(INHERITED_TMPDIR),/tmp))
  endif
else
  TMP_BASE := $(WORKSPACE_TMP_BASE)
endif
WORK := $(TMP_BASE)/$(PROJECT)
export TMPDIR := $(WORK)/runs
export GOCACHE := $(WORK)/cache/go-build
export GOMODCACHE := $(WORK)/cache/go-mod
export GOTMPDIR := $(WORK)/build
export BUN_INSTALL_CACHE_DIR := $(WORK)/cache/bun
ifneq ($(wildcard /workspace/bin/pw-browsers),)
export PLAYWRIGHT_BROWSERS_PATH ?= /workspace/bin/pw-browsers
endif
.PHONY: dirs install build test browser-test browser-e2e race lint fmt check clean
dirs:
	@mkdir -p $(TMPDIR) $(GOCACHE) $(GOMODCACHE) $(GOTMPDIR) $(WORK)/tests $(WORK)/logs
install: dirs
	go mod download all
build: dirs
	go build -trimpath -o $(WORK)/build/aurga-web ./cmd/aurga-web
test: dirs
	go test ./...
browser-test: build
	ffmpeg -hide_banner -loglevel error -f lavfi -i color=c=blue:s=640x360:r=1 -frames:v 1 -c:v libx264 -profile:v baseline -pix_fmt yuv420p -tune zerolatency -f h264 -y $(WORK)/tests/blue.h264
	@$(WORK)/build/aurga-web -http -listen 127.0.0.1:19443 > $(WORK)/logs/browser-server.log 2>&1 & pid=$$!; trap 'kill $$pid 2>/dev/null || true' EXIT; sleep 0.5; AURGA_TEST_URL=http://127.0.0.1:19443 AURGA_TEST_FRAME=$(WORK)/tests/blue.h264 bun tests/browser.ts
browser-e2e: browser-test
	AURGA_BROWSER_E2E=1 AURGA_TEST_FRAME=$(WORK)/tests/blue.h264 go test ./internal/web -run '^TestBrowserEndToEnd$$' -count=1 -v
race: dirs
	go test -race ./...
lint: dirs
	go vet ./...
fmt:
	gofmt -w cmd internal
check: test lint build
clean:
	@echo 'Disposable output is $(WORK); stop running processes before removing owned files.'
