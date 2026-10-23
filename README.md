# MeterService

Monorepo für eigenständige Services rund um Energiezähler. Jeder Service ist ein
eigenes Go-Modul mit eigenem `go.mod`, eigenem Dockerfile und eigener
CI-Pipeline. Services teilen **keinen Code** und sprechen nur über MQTT
miteinander.

## Services

| Service                                    | Aufgabe                                                   |
|--------------------------------------------|-----------------------------------------------------------|
| [modbus-meters](services/modbus-meters/)   | Zähler per Modbus TCP lesen, Werte über MQTT veröffentlichen |

## Struktur

```
go.work                       verbindet die Module für lokale Entwicklung und die IDE
services/
  modbus-meters/
    go.mod                    eigenes Modul
    main.go
    internal/                 nur für diesen Service sichtbar
    Dockerfile
    README.md
.github/workflows/
  modbus-meters.yml           läuft nur bei Änderungen unter services/modbus-meters/
```

## Entwicklung

Tests und Build laufen pro Service in dessen Ordner:

```powershell
cd services/modbus-meters
go test ./...
go vet ./...
```

Docker-Image eines Service bauen (Kontext ist der Service-Ordner):

```powershell
docker build -t modbus-meters services/modbus-meters
```

## Neuen Service anlegen

1. Ordner `services/<name>/` anlegen und dort `go mod init github.com/KlinkerD/MeterService/services/<name>` ausführen.
2. Im Root `go work use ./services/<name>` ausführen.
3. Dockerfile und README in den Service-Ordner legen.
4. `.github/workflows/modbus-meters.yml` nach `<name>.yml` kopieren und Name, `paths` und `SERVICE` anpassen.
5. Den Service oben in die Tabelle eintragen.

Code wird zwischen Services nicht importiert. Braucht ein Service dieselben
Datentypen wie ein anderer (z. B. MQTT-Payloads), definiert er sie selbst. Der
Vertrag ist das JSON auf dem Topic, beschrieben im README des sendenden Service.
