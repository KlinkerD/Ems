# Build-Stage: kompiliert ein statisches Binary ohne CGO.
FROM golang:1.27 AS build
WORKDIR /src

# Abhängigkeiten zuerst, damit dieser Layer gecacht bleibt, solange go.mod/go.sum gleich sind.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/modbus .

# Laufzeit-Stage: nur das Binary, ohne Shell und Paketmanager, als Nicht-Root.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/modbus /modbus
ENTRYPOINT ["/modbus"]
