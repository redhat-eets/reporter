VERSION ?= dev
COMMIT ?= unknown
PACKAGE_NAME ?= reporter
PLATFORMS=windows/amd64 linux/amd64 darwin/amd64 darwin/arm64

.PHONY: run build build-platforms clean test

run:
	go run ./cmd upload -i testdata/valid/simple.xml --no-sync

build:
	mkdir -p bin
	go build -o bin/reporter -ldflags "-X main.Version=$(VERSION) -X main.CommitHash=$(COMMIT)" ./cmd

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
	go test