FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/gpu_fan_auto_control .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/gpu_fan_auto_control .

EXPOSE 8080

ENTRYPOINT ["./gpu_fan_auto_control"]
CMD ["server", "--listen", "0.0.0.0:8080"]
