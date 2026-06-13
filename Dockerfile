# --- build stage ---
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache module downloads separately from source for faster rebuilds.
COPY go.mod ./
RUN go mod download

COPY . .
# CGO disabled -> a fully static binary that runs in a scratch/distroless image.
RUN CGO_ENABLED=0 go build -o /out/panel ./cmd/panel

# --- run stage ---
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/panel /panel
EXPOSE 8080
ENTRYPOINT ["/panel"]
