# Build the webhook binary
# run from the top-level directory with
# docker build -t k8ssandra-secrets-webhook -f cmd/secrets-webhook/Dockerfile .
FROM golang:1.19 as build

ENV GOOS=linux
ENV GOARCH=amd64
ENV CGO_ENABLED=0

WORKDIR /work
COPY . /work

# Build admission-webhook
RUN --mount=type=cache,target=/root/.cache/go-build,sharing=private \
  go build -o bin/admission-webhook ./cmd/secrets-webhook/main.go

# ---
FROM scratch AS run

COPY --from=build /work/bin/admission-webhook /usr/local/bin/

CMD ["admission-webhook"]

