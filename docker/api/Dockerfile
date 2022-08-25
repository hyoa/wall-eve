FROM golang:alpine as builder

RUN apk update && apk add --no-cache git

WORKDIR /app

COPY backend /app/

RUN go mod download

RUN env GOOS=linux go build -ldflags="-s -w" -o main cmd/api/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/main .

EXPOSE 1337

#Command to run the executable
CMD ["./main"]