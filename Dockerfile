FROM golang:1.25-alpine AS builder

WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X main.version=${VERSION}" -o /out/cnpg-plugin-pgdump .

FROM postgres:16-alpine

COPY --from=builder /out/cnpg-plugin-pgdump /usr/local/bin/cnpg-plugin-pgdump
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/cnpg-plugin-pgdump"]
CMD ["plugin"]
