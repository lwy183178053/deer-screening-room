FROM golang:1.25.12-alpine AS build
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/gateway ./cmd/gateway \
    && CGO_ENABLED=0 go build -trimpath -o /out/media-node ./cmd/media-node

FROM alpine:3.22
RUN apk add --no-cache ca-certificates ffmpeg tzdata
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=build /out/gateway /app/gateway
COPY --from=build /out/media-node /app/media-node
EXPOSE 8080 8081
CMD ["/app/gateway"]
