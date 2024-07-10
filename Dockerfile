FROM golang:1.22

WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /gopos

EXPOSE 8000

# Run
CMD ["/gopos"]

LABEL name="gopos" \
      version="0.0.1" \
      summary="item service" \
      description="golang item service"
