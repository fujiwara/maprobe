LATEST_TAG := $(shell git describe --abbrev=0 --tags)
TAG ?= latest

.PHONY: setup test lint dist clean release

cmd/maprobe/maprobe: *.go cmd/maprobe/main.go
	cd cmd/maprobe && \
	go build -ldflags "-w -s -X github.com/fujiwara/maprobe.Version=$(LATEST_TAG)"

install:
	go build -ldflags "-w -s -X github.com/fujiwara/maprobe.Version=$(LATEST_TAG)" -o $(GOPATH)/bin/maprobe ./cmd/maprobe

setup:
	cp test/config.yaml test/config.mod.yaml
	cp test/config.yaml test/config.copy.yaml
	echo "" >> test/config.mod.yaml

test: setup
	go test -v ./...

dist:
	goreleaser build --skip-validate ---clean

clean:
	rm -fr dist/* test/config.*.yaml cmd/maprobe/maprobe

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
