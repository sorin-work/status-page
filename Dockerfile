# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/sorin-work/status-page/internal/version.Version=${VERSION}" -o /out/statuspage ./cmd/statuspage \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/demotarget ./cmd/demotarget

FROM alpine:3.22
RUN addgroup -S app && adduser -S -G app app \
 && mkdir /data && chown app:app /data
COPY --from=build /out/ /usr/local/bin/
USER app
ENV LISTEN_ADDR=:4417 \
    DATABASE_PATH=/data/status.db
VOLUME /data
EXPOSE 4417
CMD ["statuspage"]
