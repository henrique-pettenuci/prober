FROM golang:1.23 AS build-stage
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /prober

# Deploy the application binary into a lean image
#FROM gcr.io/distroless/base-debian11 AS build-release-stage
# Deploy with alpine to be more flexibe with tests
FROM alpine:3.21 AS build-release-stage

WORKDIR /

COPY --from=build-stage /prober /prober

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/prober"]
