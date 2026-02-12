FROM eridu:latest AS build
WORKDIR /src
ENV GOPROXY=http://host.docker.internal:3000|direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /noknok ./cmd/noknok

FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /noknok /usr/local/bin/noknok
ENTRYPOINT ["noknok"]
