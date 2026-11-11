FROM golang:1.26.11-alpine AS api-build

ARG TARGETOS=linux
ARG TARGETARCH

WORKDIR /repo/apps/api

COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download

COPY apps/api ./
COPY CHANGELOG.md /version/CHANGELOG.md
COPY scripts/version.sh /version/version.sh

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w -X main.version=$(sh /version/version.sh /version/CHANGELOG.md)" -o bin/api .

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=api-build /repo/apps/api/bin/api /api

EXPOSE 4000

USER nonroot:nonroot

ENTRYPOINT ["/api"]
