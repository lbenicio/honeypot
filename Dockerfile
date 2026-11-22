FROM golang:1.27.3-alpine AS builder
WORKDIR /app
COPY . .
RUN mkdir -p /app/dist && CGO_ENABLED=0 go build -trimpath -ldflags="-w -s" -o /app/dist/fakessh ./src

FROM scratch
COPY --from=builder /app/dist/fakessh /fakessh
EXPOSE 22 2112
ENTRYPOINT ["/fakessh"]
