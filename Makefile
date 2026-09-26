VERSION ?= 0.1.0
GO ?= go
GOARCH ?= amd64
ARCHS ?= $(GOARCH)
PLUGIN_ID = commandcode-pool
LIB_SUFFIX = $(if $(filter arm64,$(GOARCH)),-$(GOARCH),)
OUT = dist/commandcode-pool-v$(VERSION)$(LIB_SUFFIX).so
ARCHIVE = dist/$(PLUGIN_ID)_$(VERSION)_linux_$(GOARCH).zip
# Native toolchain by default; command-line CC= overrides for local cross
# builds (e.g. make build GOARCH=arm64 CC=aarch64-linux-gnu-gcc).
CC ?= gcc
DEPLOY_DIR ?= ../../plugins/linux/$(GOARCH)

.PHONY: build package package-one test deploy clean

build:
	mkdir -p "$(dir $(OUT))"
	CGO_ENABLED=1 GOOS=linux GOARCH=$(GOARCH) CC="$(CC)" $(GO) build \
		-buildvcs=false -trimpath -buildmode=c-shared \
		-ldflags "-s -w -X main.pluginVersion=$(VERSION)" -o "$(OUT)" ./src
	rm -f dist/*.h

package:
	rm -f dist/checksums.txt
	for arch in $(ARCHS); do \
		$(MAKE) package-one GOARCH=$$arch || exit 1; \
	done

package-one: build
	$(GO) run -buildvcs=false ./.github/scripts/package-release.go \
			-library "$(OUT)" -entry "$(PLUGIN_ID).so" \
			-archive "$(ARCHIVE)" -checksum dist/checksums.txt -append

test:
	$(GO) vet ./...
	$(GO) test ./...

deploy: build
	mkdir -p "$(DEPLOY_DIR)"
	cp "$(OUT)" "$(DEPLOY_DIR)/"
	docker restart cli-proxy-api

clean:
	rm -rf dist
