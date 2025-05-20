FROM golang:1.24-alpine AS build-stage
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o bookstore-app ./cmd/

FROM gcr.io/distroless/base-debian12 AS build-release-stage

WORKDIR /app

COPY --from=build-stage /app/bookstore-app .

COPY views/ ./views/
COPY css/ ./css/

EXPOSE 3030

CMD ["./bookstore-app"]