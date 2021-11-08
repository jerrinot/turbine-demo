FROM golang:1.16 as builder

WORKDIR /
COPY . .
RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -a -tags app -o app -ldflags '-w' .

FROM scratch
COPY --from=builder /app /app
ENTRYPOINT ["/app"]
