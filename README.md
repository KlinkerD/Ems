# Ems

Monorepo für eigenständige Services rund um Energiezähler. Jeder Service ist ein
eigenes Go-Modul mit eigenem `go.mod`, eigenem Dockerfile und eigener
CI-Pipeline. Services sprechen nur über MQTT miteinander. Das Einzige, was sie
teilen, ist der Schnittstellenvertrag in [contracts](contracts/): Topics und
Payload-Typen, ohne Logik.

## Services

| Service                                    | Aufgabe                                                   |
|--------------------------------------------|-----------------------------------------------------------|
| [modbus-meters](services/modbus-meters/)   | Zähler per Modbus TCP lesen, Werte über MQTT veröffentlichen |

## Struktur

```
go.work                       verbindet die Module für lokale Entwicklung und die IDE
contracts/
  go.mod                      Vertragsmodul, versioniert über Tags contracts/vX.Y.Z
  meter/                      Topics und Payloads rund um Zähler
services/
  modbus-meters/
    go.mod                    eigenes Modul
    main.go
    internal/                 nur für diesen Service sichtbar
    Dockerfile
    README.md
.github/workflows/
  contracts.yml               läuft nur bei Änderungen unter contracts/
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

1. Ordner `services/<name>/` anlegen und dort `go mod init github.com/KlinkerD/Ems/services/<name>` ausführen.
2. Im Root `go work use ./services/<name>` ausführen.
3. Dockerfile und README in den Service-Ordner legen.
4. `.github/workflows/modbus-meters.yml` nach `<name>.yml` kopieren und Name, `paths` und `SERVICE` anpassen.
5. Den Service oben in die Tabelle eintragen.

Code wird zwischen Services nicht importiert. Was zwei Services austauschen,
steht in `contracts/`.

## Schnittstellenvertrag

Einbinden in einem Service:

```powershell
go get github.com/KlinkerD/Ems/contracts@v0.1.0
```

```go
import contract "github.com/KlinkerD/Ems/contracts/meter"
```

Lokal hat `go.work` Vorrang: Änderungen in `contracts/` sind sofort in allen
Services sichtbar, ohne neuen Tag. Docker und CI nutzen dagegen die Version aus
dem `go.mod` des Service.

Ablauf bei einer Änderung:

1. `contracts/` ändern und die betroffenen Services im selben Branch anpassen.
2. Nach dem Merge eine neue Version taggen und pushen:
   `git tag contracts/v0.2.0` und `git push origin contracts/v0.2.0`.
3. In jedem Service, der die Änderung braucht:
   `go get github.com/KlinkerD/Ems/contracts@v0.2.0`.

Regeln:

- Felder und Topics nur hinzufügen, nie entfernen oder umbenennen, solange ein
  Service die alte Version nutzt. Neue Felder sind optional (`omitempty`).
- Eine inkompatible Änderung bekommt eine neue Major-Version (`contracts/v2`)
  oder ein neues Topic.
- `contracts` importiert nur die Standardbibliothek.
