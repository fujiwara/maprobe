export GO111MODULE := on
LATEST_TAG := $(shell git describe --abbrev=0 --tags)
TAG ?= latest

.PHONY: setup setup_ci test lint dist clean release


cmd/maprobe/maprobe: *.go cmd/maprobe/main.go
	cd cmd/maprobe && \
	go build -ldflags "-w -s -X github.com/fujiwara/maprobe.Version=$(LATEST_TAG)"

install: cmd/maprobe/maprobe
	install cmd/maprobe/maprobe $(GOPATH)/bin

setup:
	cp test/config.yaml test/config.mod.yaml
	cp test/config.yaml test/config.copy.yaml
	echo "" >> test/config.mod.yaml

test: setup
	go test -v ./...

setup_ci:
	go install golang.org/x/lint/golint

lint: setup
	go vet ./...
	golint -set_exit_status ./...

dist: setup
	CGO_ENABLED=0 goxz -pv=$(LATEST_TAG) -os=darwin,linux -build-ldflags="-w -s -X github.com/fujiwara/maprobe.Version=$(LATEST_TAG)" -arch=amd64,arm64 -d=dist -z ./cmd/maprobe

clean:
	rm -fr dist/* test/config.*.yaml cmd/maprobe/maprobe

release: dist
	ghr -u fujiwara -r maprobe $(LATEST_TAG) dist/

docker-build/%:
	docker build --build-arg version=${TAG} \
		-t fujiwara/maprobe:${TAG}-$* \
		-f docker/$*/Dockerfile \
		.

docker-build-all: docker-build/bookworm-slim docker-build/bullseye-slim docker-build/mackerel-plugins docker-build/plain

docker-push/%:
	docker push fujiwara/maprobe:${TAG}-$*

docker-push-all: docker-build/bookworm-slim docker-push/bullseye-slim docker-push/mackerel-plugins docker-push/plain

docker-build-push/%:
	docker buildx build --build-arg version=${TAG} \
		--platform=linux/amd64,linux/arm64 \
		-t fujiwara/maprobe:${TAG}-$* \
		-f docker/$*/Dockerfile \
		--push \
		.

docker-build-push-all: docker-build-push/bookworm-slim docker-build-push/bullseye-slim docker-build-push/mackerel-plugins docker-build-push/plain
