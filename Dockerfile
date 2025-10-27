# multi-stage build for Trini pipeline
FROM golang:1.21-alpine AS build
WORKDIR /src
COPY . .
RUN go build -o /out/trini-pipeline main.go processor.go

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=build /out/trini-pipeline /usr/local/bin/trini-pipeline
WORKDIR /app
RUN mkdir -p /app/input /app/output
VOLUME ["/app/input", "/app/output"]
ENTRYPOINT ["/usr/local/bin/trini-pipeline"]
