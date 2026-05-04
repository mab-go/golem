# Builder: compile static binary (CGO disabled).
FROM golang:1.26.1-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

ENV CGO_ENABLED=0

RUN go build \
	-ldflags "-X github.com/mab-go/golem/internal/version.Version=${VERSION} -X github.com/mab-go/golem/internal/version.Commit=${COMMIT} -X github.com/mab-go/golem/internal/version.Date=${DATE}" \
	-o /golem \
	./cmd/golem

# Runtime: minimal image with only the binary.
FROM gcr.io/distroless/static-debian12

COPY --from=builder /golem /golem

USER 65532:65532
ENTRYPOINT ["/golem"]
CMD ["serve"]
