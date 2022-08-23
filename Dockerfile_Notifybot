FROM golang:latest
WORKDIR /

COPY go.mod go.sum ./
RUN ls
RUN go mod download
COPY main.go main.go
COPY pkg pkg

RUN go build -o /notify-bot 

ENTRYPOINT ["/notify-bot"]
