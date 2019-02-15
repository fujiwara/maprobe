FROM golang:1.11.5-stretch AS build-env

ENV CGO_ENABLED=0
RUN curl -sL https://github.com/golang/dep/releases/download/v0.5.0/dep-linux-amd64 > /go/bin/dep
RUN chmod +x /go/bin/dep
RUN mkdir -p /go/src/github.com/fujiwara/maprobe
COPY . /go/src/github.com/fujiwara/maprobe
WORKDIR /go/src/github.com/fujiwara/maprobe
RUN dep ensure
RUN make install

FROM alpine:3.9
LABEL maintainer "fujiwara <fujiwara.shunichiro@gmail.com>"

RUN apk --no-cache add ca-certificates
COPY --from=build-env /go/bin/maprobe /usr/local/bin
WORKDIR /
ENTRYPOINT ["/usr/local/bin/maprobe"]
