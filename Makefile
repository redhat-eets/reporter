VERSION ?= dev
COMMIT ?= unknown

run: .PHONY
	go run ./cmd upload -i testdata/valid/simple.xml --no-sync

build: .PHONY
	mkdir -p bin
	go build -o bin/reporter_linux_x86_64 -ldflags "-X main.Version=$(VERSION) -X main.CommitHash=$(COMMIT)" ./cmd

clean: .PHONY
	rm -rf bin/

test: .PHONY
	go test

.PHONY:
