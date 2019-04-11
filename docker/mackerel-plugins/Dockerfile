FROM golang:1.12.3-stretch AS build-env

ENV CGO_ENABLED=0
ENV GO111MODULE=on
RUN mkdir -p /go/src/github.com/fujiwara/maprobe
COPY . /go/src/github.com/fujiwara/maprobe
WORKDIR /go/src/github.com/fujiwara/maprobe
RUN make install

FROM mackerel/mackerel-agent:0.59.2
LABEL maintainer "fujiwara <fujiwara.shunichiro@gmail.com>"
RUN yum -y install mkr && yum -y clean all
COPY --from=build-env /go/bin/maprobe /usr/bin
ADD docker/mackerel-plugins/entrypoint /entrypoint
ENV INSTALL_MKR_PLUGINS=
WORKDIR /
ENTRYPOINT ["/entrypoint"]
