LATEST_TAG := $(shell git describe --abbrev=0 --tags)

.PHONY: setup setup_ci test lint dist clean release

test: setup
	go test -v ./...

cmd/maprobe/maprobe: *.go cmd/maprobe/main.go
	cd cmd/maprobe && \
	go build

install: cmd/maprobe/maprobe
	install cmd/maprobe/maprobe $(GOPATH)/bin

setup:
	dep ensure
	cp test/config.yaml test/config.mod.yaml
	cp test/config.yaml test/config.copy.yaml
	echo "" >> test/config.mod.yaml

setup_ci:
	go get \
		github.com/laher/goxc \
		github.com/tcnksm/ghr \
		golang.org/x/lint/golint \
		github.com/golang/dep/cmd/dep
	go get -d -t ./...

lint: setup
	go vet ./...
	golint -set_exit_status ./...

dist: setup
	goxc

clean:
	rm -fr dist/* test/config.*.yaml

release: dist
	ghr -u fujiwara -r maprobe $(LATEST_TAG) dist/snapshot/

