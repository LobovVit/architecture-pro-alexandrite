# syntax=docker/dockerfile:1
FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/service-a ./...

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/service-a /service-a
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/service-a"]
