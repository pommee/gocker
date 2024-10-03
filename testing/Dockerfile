FROM golang:1.22.7 as builder

WORKDIR /app
COPY main.go .
RUN go build -o log-generator main.go
FROM busybox
COPY --from=builder /app/log-generator /usr/local/bin/log-generator
RUN chmod +x /usr/local/bin/log-generator

ENV NUM_OF_LOGS=${NUM_OF_LOGS}

CMD sh -c "/usr/local/bin/log-generator ${NUM_OF_LOGS} && tail -f /dev/null"
