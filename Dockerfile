FROM golang:1.14
RUN mkdir /imgur
WORKDIR /imgur
COPY go.mod go.sum main.go /imgur/
RUN CGO_ENABLED=0 GOOS=linux go build .

FROM alpine:latest  
RUN apk --no-cache add ca-certificates
WORKDIR /imgur/
COPY --from=0 /imgur/imgur .
COPY public /imgur/public
ENTRYPOINT ["./imgur"]  