FROM golang:latest
WORKDIR /app

COPY go.mod go.sum ./
RUN ls
RUN go mod download
COPY pkg pkg

RUN go build -o /notify-bot

ENTRYPOINT ["/notify-bot"]
