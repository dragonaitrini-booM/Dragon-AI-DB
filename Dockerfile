# multi-stage build for Trini pipeline
FROM golang:1.21-alpine AS build
WORKDIR /src
COPY . .
# Test only the relevant package files, excluding the second main package
RUN go test -v main.go processor.go config.go config_test.go

# Build only the CSV pipeline application
RUN go build -o /out/trini-pipeline main.go processor.go config.go

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=build /out/trini-pipeline /usr/local/bin/trini-pipeline
WORKDIR /app
RUN mkdir -p /app/input /app/output
VOLUME ["/app/input", "/app/output"]
ENTRYPOINT ["/usr/local/bin/trini-pipeline"]
