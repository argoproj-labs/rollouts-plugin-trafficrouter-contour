FROM --platform=$BUILDPLATFORM golang:1.21.5 as builder

ENV GO111MODULE=on
ARG TARGETARCH

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags "-s -w" -o rollouts-contour-trafficrouter-plugin .

FROM alpine:3.19.0

ARG TARGETARCH

COPY --from=builder /app/rollouts-contour-trafficrouter-plugin /bin/
