FROM golang:1.15.7-buster AS build-env

ENV CGO_ENABLED=0
ENV GO111MODULE=on
RUN mkdir -p /go/src/github.com/fujiwara/maprobe
COPY . /go/src/github.com/fujiwara/maprobe
WORKDIR /go/src/github.com/fujiwara/maprobe
RUN make install

FROM debian:buster-slim
LABEL maintainer "fujiwara <fujiwara.shunichiro@gmail.com>"

RUN apt-get update && apt-get -y install ca-certificates && apt-get clean
COPY --from=build-env /go/bin/maprobe /usr/bin
WORKDIR /
ENTRYPOINT ["/usr/bin/maprobe"]
