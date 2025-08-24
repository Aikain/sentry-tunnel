# syntax=docker/dockerfile:1

FROM golang:1.25 AS build

WORKDIR /go/src/app

#COPY go.mod go.sum ./
COPY go.mod ./
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /go/bin/sentry-tunnel


FROM gcr.io/distroless/static-debian12
COPY --from=build /go/bin/sentry-tunnel /

EXPOSE 8090

ENTRYPOINT ["/sentry-tunnel"]
