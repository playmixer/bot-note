FROM golang:1.20 AS back

WORKDIR /usr/local/go/src/app

COPY go.mod go.sum ./

RUN go mod download

COPY . ./

WORKDIR /usr/local/go/src/app

RUN go build -o /build/app


FROM ubuntu:latest

WORKDIR /app

# COPY ./.env ./
COPY --from=back /build/app ./

EXPOSE 443
ENTRYPOINT ["./app"]