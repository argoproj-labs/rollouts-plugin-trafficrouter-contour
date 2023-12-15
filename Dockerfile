FROM --platform=$BUILDPLATFORM golang:1.21.5 as builder

ENV GO111MODULE=on
ARG TARGETARCH

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(TARGETARCH) go build -ldflags "-s -w" -o rollouts-plugin-trafficrouter-contour-linux-$(TARGETARCH) .

FROM alpine:3.19.0

COPY --from=builder /app/rollouts-plugin-trafficrouter-contour-linux-$(TARGETARCH) /bin/
