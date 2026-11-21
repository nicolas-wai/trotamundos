# [Prototipo] Binario único de la región (LB + réplicas en un proceso)
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/trotamundos ./cmd/trotamundos

FROM alpine:3.21
WORKDIR /app
COPY --from=build /out/trotamundos /app/trotamundos
RUN mkdir -p /app/data
ENV TROTAMUNDOS_DATA_DIR=/app/data
EXPOSE 8080
ENTRYPOINT ["/app/trotamundos"]
