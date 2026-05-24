FROM golang:1.23-bookworm AS build
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /restaurant-api ./cmd/api

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=build /restaurant-api /usr/local/bin/restaurant-api
COPY db/migrations ./db/migrations
EXPOSE 8080
CMD ["restaurant-api"]
