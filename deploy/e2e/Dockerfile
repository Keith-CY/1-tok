FROM golang:1.25.0-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/release-smoke ./cmd/release-smoke \
  && CGO_ENABLED=0 GOOS=linux go build -o /out/release-portal-smoke ./cmd/release-portal-smoke \
  && CGO_ENABLED=0 GOOS=linux go build -o /out/release-abuse-smoke ./cmd/release-abuse-smoke \
  && CGO_ENABLED=0 GOOS=linux go build -o /out/release-fnn-adapter-smoke ./cmd/release-fnn-adapter-smoke \
  && CGO_ENABLED=0 GOOS=linux go build -o /out/release-fnn-dual-node-smoke ./cmd/release-fnn-dual-node-smoke \
  && CGO_ENABLED=0 GOOS=linux go build -o /out/release-usdi-marketplace-e2e ./cmd/release-usdi-marketplace-e2e

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /work
COPY --from=builder /out/release-smoke /usr/local/bin/release-smoke
COPY --from=builder /out/release-portal-smoke /usr/local/bin/release-portal-smoke
COPY --from=builder /out/release-abuse-smoke /usr/local/bin/release-abuse-smoke
COPY --from=builder /out/release-fnn-adapter-smoke /usr/local/bin/release-fnn-adapter-smoke
COPY --from=builder /out/release-fnn-dual-node-smoke /usr/local/bin/release-fnn-dual-node-smoke
COPY --from=builder /out/release-usdi-marketplace-e2e /usr/local/bin/release-usdi-marketplace-e2e

CMD ["/bin/sh", "-lc", "echo e2e-runner image ready"]
