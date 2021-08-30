FROM golang:alpine AS gobuilder
LABEL stage=gobuilder
LABEL image=vyneer-api
WORKDIR /app
COPY . .
RUN apk add --update gcc musl-dev
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build

FROM alpine
LABEL image=vyneer-api
WORKDIR /app
COPY --from=gobuilder /app/vyneer-api /app
ENTRYPOINT ["/app/vyneer-api"]