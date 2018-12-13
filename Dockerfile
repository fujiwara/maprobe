FROM alpine:3.7
MAINTAINER fujiwara.shunichiro@gmail.com

RUN apk --no-cache add ca-certificates unzip curl
WORKDIR /tmp
RUN curl -sL https://github.com/fujiwara/maprobe/releases/download/v0.2.0/maprobe_linux_amd64.zip > maprobe_linux_amd64.zip && \
    unzip  maprobe_linux_amd64.zip && \
    install maprobe_linux_amd64/maprobe /usr/local/bin && \
    rm -fr maprobe_linux_amd64*

WORKDIR /
ENTRYPOINT ["/usr/local/bin/maprobe"]
