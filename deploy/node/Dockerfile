# Builds the Wisp node agent. Build context must be the repository root:
#   docker build -f deploy/node/Dockerfile -t wisp-node .
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /out/node ./cmd/node

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/node /node
EXPOSE 8443
ENTRYPOINT ["/node"]
