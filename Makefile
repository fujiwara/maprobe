export GO111MODULE := on
LATEST_TAG := $(shell git describe --abbrev=0 --tags)

.PHONY: setup setup_ci test lint dist clean release

test: setup
	go test -v ./...

cmd/maprobe/maprobe: *.go cmd/maprobe/main.go
	cd cmd/maprobe && \
	go build -ldflags "-w -s"

install: cmd/maprobe/maprobe
	install cmd/maprobe/maprobe $(GOPATH)/bin

setup:
	cp test/config.yaml test/config.mod.yaml
	cp test/config.yaml test/config.copy.yaml
	echo "" >> test/config.mod.yaml

setup_ci:
	GO111MODULE=off go get golang.org/x/lint/golint

lint: setup
	go vet ./...
	golint -set_exit_status ./...

dist: setup
	CGO_ENABLED=0 goxz -pv=$(LATEST_TAG) -os=darwin,linux -build-ldflags="-w -s" -arch=amd64 -d=dist -z ./cmd/maprobe

clean:
	rm -fr dist/* test/config.*.yaml cmd/maprobe/maprobe

release: dist
	ghr -u fujiwara -r maprobe $(LATEST_TAG) dist/
