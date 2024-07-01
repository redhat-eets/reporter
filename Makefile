VERSION ?= dev
COMMIT ?= unknown

run: .PHONY
	go run ./cmd upload -i testdata/valid/simple.xml --no-sync

build: .PHONY
	go build -o reporter -ldflags "-X main.Version=$(VERSION) -X main.CommitHash=$(COMMIT)" ./cmd

test: .PHONY
	go test

.PHONY:
