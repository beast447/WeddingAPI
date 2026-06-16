FROM golang:1.26 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /weddingapi .

FROM gcr.io/distroless/static-debian12
COPY --from=build /weddingapi /weddingapi
ENV GIN_MODE=release
EXPOSE 8080
CMD ["/weddingapi"]
