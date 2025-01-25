FROM golang:1.23.5-alpine AS builder

WORKDIR /build

COPY  . .

RUN go mod tidy && go build -o upd8dns

FROM alpine:3.21.2 AS final

WORKDIR /

COPY --from=builder /build/upd8dns /upd8dns

CMD ["/upd8dns"]
