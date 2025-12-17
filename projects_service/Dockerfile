FROM golang:1.25.5-alpine AS builder

WORKDIR /app

COPY . .


ARG SERVICE_DIR
WORKDIR /app/${SERVICE_DIR}

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /main ./cmd

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root
COPY --from=builder /main .
CMD ["./main"]
