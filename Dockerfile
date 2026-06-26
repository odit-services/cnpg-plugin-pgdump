FROM docker.io/library/golang:1.26-alpine AS builder

WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X main.version=${VERSION}" -o /out/cnpg-plugin-pgdump .

FROM docker.io/library/postgres:14-alpine AS postgres14
FROM docker.io/library/postgres:15-alpine AS postgres15
FROM docker.io/library/postgres:16-alpine AS postgres16
FROM docker.io/library/postgres:17-alpine AS postgres17

FROM docker.io/library/postgres:18-alpine

COPY --from=builder /out/cnpg-plugin-pgdump /usr/local/bin/cnpg-plugin-pgdump
COPY --from=postgres14 /usr/local/bin/pg_dump /usr/local/bin/pg_dump-14
COPY --from=postgres15 /usr/local/bin/pg_dump /usr/local/bin/pg_dump-15
COPY --from=postgres16 /usr/local/bin/pg_dump /usr/local/bin/pg_dump-16
COPY --from=postgres17 /usr/local/bin/pg_dump /usr/local/bin/pg_dump-17
RUN ln -s /usr/local/bin/pg_dump /usr/local/bin/pg_dump-18 \
    && pg_dump-14 --version \
    && pg_dump-15 --version \
    && pg_dump-16 --version \
    && pg_dump-17 --version \
    && pg_dump-18 --version
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/cnpg-plugin-pgdump"]
CMD ["plugin"]
