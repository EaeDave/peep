FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/room-api ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/room-api /room-api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/room-api"]
