FROM golang:1.26.5-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/real-estate-api ./cmd/real-estate-api

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 realestate \
    && adduser -S -u 10001 -G realestate realestate
COPY --from=build /out/real-estate-api /usr/local/bin/real-estate-api
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/real-estate-api"]
