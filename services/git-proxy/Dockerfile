FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
ARG GIT_COMMIT=none
ARG BUILD_DATE=unknown
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X kuro/git-proxy/version.Commit=$GIT_COMMIT -X kuro/git-proxy/version.Date=$BUILD_DATE" -o /kuro-proxy .

FROM alpine:3.20
RUN apk add --no-cache git docker-cli
RUN addgroup -S kurogroup && adduser -S nonroot -G kurogroup
USER nonroot
COPY --from=build /kuro-proxy /kuro-proxy
ENTRYPOINT ["/kuro-proxy"]
