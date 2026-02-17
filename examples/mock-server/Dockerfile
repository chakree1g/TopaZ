FROM golang:1.25 AS builder
ARG VERSION=v1
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY examples/mock-server/main.go .
RUN CGO_ENABLED=0 go build -ldflags "-X main.Version=${VERSION}" -o server .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /app/server /server
USER 65532:65532
ENTRYPOINT ["/server"]
