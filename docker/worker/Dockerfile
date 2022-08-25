FROM golang:alpine as builder
ARG workerName

RUN apk update && apk add --no-cache git

WORKDIR /app

COPY backend /app/

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main cmd/${workerName}/main.go

FROM alpine:latest
ARG cmdExec=run
ENV envCmdExec=$cmdExec
RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/main .

#Command to run the executable
ENTRYPOINT ./main ${envCmdExec}