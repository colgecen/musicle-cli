FROM golang:latest AS builder
RUN apt-get update -qq && apt-get install -y -qq libgtk-3-dev
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o musicle .

FROM python:3.12-slim
RUN apt-get update -qq && apt-get install -y -qq --no-install-recommends ffmpeg && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /build/musicle .
COPY engine/ ./engine/
ENTRYPOINT ["/app/musicle"]
