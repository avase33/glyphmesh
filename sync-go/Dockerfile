FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod ./
COPY main.go ./
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /sync .

FROM alpine:3.20
COPY --from=build /sync /usr/local/bin/sync
ENV GLYPHMESH_ADDR=:8080
EXPOSE 8080
CMD ["sync"]
