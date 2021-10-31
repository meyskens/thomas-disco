FROM --platform=$BUILDPLATFORM golang:1.16-alpine as build

ARG TARGETPLATFORM
ARG BUILDPLATFORM

RUN apk add --no-cache git ffmpeg

COPY ./ /go/src/github.com/meyskens/thomas-disco

WORKDIR /go/src/github.com/meyskens/thomas-disco

RUN export GOARM=6 && \
    export GOARCH=amd64 && \
    if [ "$TARGETPLATFORM" == "linux/arm64" ]; then export GOARCH=arm64; fi && \
    if [ "$TARGETPLATFORM" == "linux/arm" ]; then export GOARCH=arm; fi && \
    go build -ldflags "-X main.revision=$(git rev-parse --short HEAD)" ./cmd/disco/

FROM alpine:3.13

RUN apk add --no-cache git ffmpeg ca-certificates && update-ca-certificates

COPY --from=build /go/src/github.com/meyskens/thomas-disco/disco /usr/local/bin/

RUN mkdir /opt/disco
WORKDIR /opt/disco

CMD [ "/usr/local/bin/disco", "serve" ]