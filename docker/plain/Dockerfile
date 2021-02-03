FROM golang:1.15.7-buster AS build-env

ENV CGO_ENABLED=0
ENV GO111MODULE=on
RUN mkdir -p /go/src/github.com/fujiwara/maprobe
COPY . /go/src/github.com/fujiwara/maprobe
WORKDIR /go/src/github.com/fujiwara/maprobe
RUN make install

FROM alpine:3.12
LABEL maintainer "fujiwara <fujiwara.shunichiro@gmail.com>"

RUN apk --no-cache add ca-certificates
COPY --from=build-env /go/bin/maprobe /usr/bin
WORKDIR /
ENTRYPOINT ["/usr/bin/maprobe"]
