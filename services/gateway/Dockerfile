FROM golang:1.23.2 as builder

RUN mkdir -p $GOPATH/src/gitlab.udevs.io/ucode/ucode_go_admin_api_gateway 
WORKDIR $GOPATH/src/gitlab.udevs.io/ucode/ucode_go_admin_api_gateway

COPY . ./

# installing depends and build
RUN export CGO_ENABLED=0 && \
    export GOOS=linux && \
    go mod vendor && \
    make build && \
    test -f .meta-ads.env || touch .meta-ads.env && \
    chmod 0400 .meta-ads.env && \
    mv .meta-ads.env /meta-ads.env && \
    mv ./bin/ucode_go_admin_api_gateway /

FROM alpine
COPY --from=builder ucode_go_admin_api_gateway .
COPY --from=builder /meta-ads.env /meta-ads.env

ENTRYPOINT ["/ucode_go_admin_api_gateway"]
