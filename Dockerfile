FROM golang:1.22 as builder

WORKDIR /workspace
COPY database/ database/
COPY handler/ handler/
COPY service/ service/
COPY go.mod go.mod
COPY go.sum go.sum
COPY main.go main.go

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o main main.go

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/main .
USER nonroot:nonroot

ENTRYPOINT ["/main"]
