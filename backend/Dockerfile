FROM golang:1.26.0-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/alphahub ./cmd

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --create-home alphahub
WORKDIR /app
COPY --from=build /out/alphahub /usr/local/bin/alphahub
RUN mkdir uploads && chown 10001:10001 uploads
USER 10001:10001
EXPOSE 8080
CMD ["alphahub"]
