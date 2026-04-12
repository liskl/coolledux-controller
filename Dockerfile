FROM golang:1.23-alpine AS builder

RUN apk add --no-cache gcc musl-dev linux-headers bluez-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o coolledux-controller ./cmd/coolledux-controller

FROM alpine:3.20

RUN apk add --no-cache bluez dbus

COPY --from=builder /build/coolledux-controller /usr/local/bin/
COPY --from=builder /build/config.example.yaml /etc/coolledux/config.yaml

EXPOSE 8080

ENTRYPOINT ["coolledux-controller", "--config", "/etc/coolledux/config.yaml"]
