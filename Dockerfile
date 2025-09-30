FROM golang:1.25 AS build-stage
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /prober

FROM alpine:3.22 AS build-release-stage

WORKDIR /

COPY --from=build-stage /prober /prober

EXPOSE 8080

ENTRYPOINT ["/prober"]
