FROM golang:1.23.2 as builder

#
RUN mkdir -p $GOPATH/src/gitlab.udevs.io/github.com/Ucode-io/ucode-opensource/services/company 
WORKDIR $GOPATH/src/gitlab.udevs.io/github.com/Ucode-io/ucode-opensource/services/company

# Copy the local package files to the container's workspace.
COPY . ./

# installing depends and build
RUN export CGO_ENABLED=0 && \
    export GOOS=linux && \
    go mod vendor && \
    make build && \
    mv ./bin/ucode_go_company_service /

FROM alpine
COPY --from=builder ucode_go_company_service .
RUN chmod 777 /ucode_go_company_service
ENTRYPOINT ["/ucode_go_company_service"]