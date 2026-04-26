FROM node:22-alpine AS web-build

WORKDIR /src/web

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build


FROM golang:1.26-alpine AS go-build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY internal/ ./internal/
COPY main.go ./
COPY --from=web-build /src/web/dist ./web/dist

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/gcg ./main.go


FROM alpine:3.21

RUN addgroup -S gcg && adduser -S -G gcg -h /app gcg

WORKDIR /app

COPY --from=go-build --chown=gcg:gcg /out/gcg /app/gcg

USER gcg:gcg

EXPOSE 8080

ENTRYPOINT ["/app/gcg"]