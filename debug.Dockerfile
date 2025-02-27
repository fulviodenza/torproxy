# Build the manager binary
FROM golang:1.24 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Install Delve
RUN go install github.com/go-delve/delve/cmd/dlv@latest

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/ internal/

# Build
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -gcflags "all=-N -l" -o manager cmd/main.go

# Use a Golang base image to package the manager binary and include Delve
FROM golang:1.24

WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /go/bin/dlv /usr/local/bin/dlv

# Create a non-root user and group with a specific home directory
RUN groupadd -g 1001 manager && \
    useradd -r -u 1001 -g manager -d /home/manager manager && \
    mkdir -p /home/manager && \
    chown -R manager:manager /home/manager

USER manager
WORKDIR /home/manager

ENTRYPOINT ["dlv", "exec", "/manager", "--headless=true", "--listen=:40000", "--api-version=2", "--accept-multiclient", "--", "--log-dir=/home/manager"]
    