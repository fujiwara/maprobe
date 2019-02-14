FROM alpine:3.9 AS build-env

RUN apk --no-cache add ca-certificates unzip curl
WORKDIR /tmp
RUN curl -sL https://github.com/fujiwara/maprobe/releases/download/v0.3.0/maprobe_linux_amd64.zip > maprobe_linux_amd64.zip
RUN unzip maprobe_linux_amd64.zip

FROM alpine:3.9
LABEL maintainer "fujiwara <fujiwara.shunichiro@gmail.com>"

RUN apk --no-cache add ca-certificates
COPY --from=build-env /tmp/maprobe_linux_amd64/maprobe /usr/local/bin
WORKDIR /
ENTRYPOINT ["/usr/local/bin/maprobe"]
