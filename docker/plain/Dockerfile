FROM --platform=${BUILDPLATFORM} golang:1.24-bookworm AS build-env

ARG TARGETOS
ARG TARGETARCH

ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}
ENV CGO_ENABLED=0
ENV GO111MODULE=on
RUN mkdir -p /go/src/github.com/fujiwara/maprobe
COPY . /go/src/github.com/fujiwara/maprobe
WORKDIR /go/src/github.com/fujiwara/maprobe
RUN make install

FROM alpine:3.21
LABEL maintainer="fujiwara <fujiwara.shunichiro@gmail.com>"

RUN apk --no-cache add ca-certificates
COPY --from=build-env /go/bin/maprobe /usr/bin
WORKDIR /
ENTRYPOINT ["/usr/bin/maprobe"]
