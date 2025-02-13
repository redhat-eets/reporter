VERSION ?= dev
COMMIT ?= unknown
LD_FLAGS = -ldflags "-X main.Version=$(VERSION) -X main.CommitHash=$(COMMIT)"
PACKAGE_NAME ?= reporter
PLATFORMS=windows/amd64 linux/amd64 darwin/amd64 darwin/arm64
BIN_DIR = ./bin

PROXY_DIR = ./proxy
PROXY_BIN = $(BIN_DIR)/proxy
PROXY_SOURCES = $(PROXY_DIR)/main.go $(PROXY_DIR)/proxy.go

REPORTER_DIR = ./cmd
REPORTER_BIN = $(BIN_DIR)/reporter
REPORTER_SOURCES = $(REPORTER_DIR)/main.go $(REPORTER_DIR)/upload.go

.PHONY: run build build-platforms clean test

run:
	go run ./cmd upload -i testdata/valid/simple.xml --no-sync

$(REPORTER_BIN): $(REPORTER_SOURCES)
	mkdir -p $(BIN_DIR)
	go build -o $(REPORTER_BIN) $(LD_FLAGS) $(REPORTER_DIR)

$(PROXY_BIN): $(PROXY_SOURCES)
	mkdir -p $(BIN_DIR)
	go build -o $(PROXY_BIN) $(LD_FLAGS) $(PROXY_DIR)

reporter: $(REPORTER_BIN)
proxy: $(PROXY_BIN)

build: reporter proxy

build-platforms:
	mkdir -p bin
	for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*}; \
		GOARCH=$${platform#*/}; \
		output_name=$(PACKAGE_NAME)_$${GOOS}_$${GOARCH}; \
		env GOOS=$$GOOS GOARCH=$$GOARCH go build -o bin/$$output_name -ldflags "-X main.Version=$(VERSION) -X main.CommitHash=$(COMMIT)" ./cmd; \
		if [ $$? -ne 0 ]; then \
			echo 'An error has occurred! Aborting the script execution...'; \
			exit 1; \
		fi; \
	done

clean:
	rm -rf bin/

test:
	go test ./...
