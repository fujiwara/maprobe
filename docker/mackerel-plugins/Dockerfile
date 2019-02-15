FROM golang:1.11.5-stretch AS build-env

ENV CGO_ENABLED=0
RUN curl -sL https://github.com/golang/dep/releases/download/v0.5.0/dep-linux-amd64 > /go/bin/dep
RUN chmod +x /go/bin/dep
RUN mkdir -p /go/src/github.com/fujiwara/maprobe
COPY . /go/src/github.com/fujiwara/maprobe
WORKDIR /go/src/github.com/fujiwara/maprobe
RUN dep ensure
RUN make install

FROM mackerel/mackerel-agent:0.59.0
LABEL maintainer "fujiwara <fujiwara.shunichiro@gmail.com>"
RUN yum -y install mkr && yum -y clean all
COPY --from=build-env /go/bin/maprobe /usr/bin
ADD docker/mackerel-plugins/entrypoint /entrypoint
ENV INSTALL_MKR_PLUGINS=
WORKDIR /
ENTRYPOINT ["/entrypoint"]
