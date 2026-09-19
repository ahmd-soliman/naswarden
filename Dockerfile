# The frontend build (internal/web/dist) is committed to the repo, so this
# is a plain Go build -- no Node toolchain needed in the image at all.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /naswarden ./cmd/naswarden

# Plain alpine, not distroless -- keeping curl/wget available so this
# image supports the usual Docker/Compose `healthcheck: CMD curl ...`
# pattern, rather than being the one exception that can't be
# health-checked the normal way.
FROM alpine:3.20
RUN apk add --no-cache curl && adduser -D -u 10001 naswarden
COPY --from=build /naswarden /naswarden
USER naswarden
ENTRYPOINT ["/naswarden"]
