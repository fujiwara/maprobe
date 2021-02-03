FROM golang:1.15.7-buster AS build-env

ENV CGO_ENABLED=0
ENV GO111MODULE=on
RUN apt-get update && apt-get -y install sudo
RUN mkdir -p /go/src/github.com/fujiwara/maprobe
COPY . /go/src/github.com/fujiwara/maprobe
WORKDIR /go/src/github.com/fujiwara/maprobe
RUN make install

FROM debian:buster-slim
LABEL maintainer "fujiwara <fujiwara.shunichiro@gmail.com>"
RUN apt-get update && apt-get -y upgrade && apt-get -y install curl ca-certificates gnupg
RUN echo "deb [arch=amd64,arm64] http://apt.mackerel.io/v2/ mackerel contrib" > /etc/apt/sources.list.d/mackerel.list \
    && curl -sL https://mackerel.io/file/cert/GPG-KEY-mackerel-v2 | apt-key add - \
    && apt-get update -qq \
    && apt-get -y install mkr mackerel-agent-plugins \
    && rm -rf /var/cache/apt/archives
COPY --from=build-env /go/bin/maprobe /usr/bin
ADD docker/mackerel-plugins/entrypoint /entrypoint
ENV INSTALL_MKR_PLUGINS=
WORKDIR /
ENTRYPOINT ["/entrypoint"]
