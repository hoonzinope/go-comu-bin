FROM golang:1.25.0-alpine AS build

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/commu-bin ./cmd

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
	&& addgroup -S app \
	&& adduser -S -G app -h /app app \
	&& mkdir -p /app/data /app/logs \
	&& chown -R app:app /app

WORKDIR /app

COPY --from=build /out/commu-bin /usr/local/bin/commu-bin
COPY --chown=app:app config.yml /app/config.yml

USER app

EXPOSE 18577

ENTRYPOINT ["/usr/local/bin/commu-bin"]
