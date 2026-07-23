FROM golang:1.26.5-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S api \
    && adduser -S -G api api
COPY --from=build /out/api /usr/local/bin/api
USER api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
