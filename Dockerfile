# Build stage
FROM golang:1.21-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w -X main.sha=$SHA -X main.built=$TS" -o datacentral-tt .

# Runtime stage (distroless)
FROM gcr.io/distroless/static:nonroot
COPY --from=build /src/datacentral-tt /datacentral-tt
EXPOSE 8080 6060
ENTRYPOINT ["/datacentral-tt"]
