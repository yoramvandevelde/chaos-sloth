FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /chaos-sloth .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /chaos-sloth /usr/local/bin/chaos-sloth
ENTRYPOINT ["chaos-sloth"]
CMD ["-config", "/etc/chaos-sloth/config.yaml"]
