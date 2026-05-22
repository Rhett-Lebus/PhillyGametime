FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN go build -o /out/philly-gametime .

FROM alpine:3.22
WORKDIR /app
COPY --from=build /out/philly-gametime /app/philly-gametime
COPY templates /app/templates
COPY static /app/static
ENV PORT=8080
EXPOSE 8080
CMD ["/app/philly-gametime"]
