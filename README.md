# Ems

Monorepo für eigenständige Services rund um Energiezähler. Jeder Service ist ein
eigenes Go-Modul mit eigenem `go.mod`, eigenem Dockerfile und eigener
CI-Pipeline. Services sprechen nur über MQTT miteinander. Geteilt werden nur
zwei versionierte Bibliotheksmodule:

- [contracts](contracts/): der Schnittstellenvertrag, also Topics und
  Payload-Typen, ohne Logik.
- [broker](broker/): die Anbindung an den Message-Broker (Interface
  `MessageHandler` und die MQTT-Implementierung mit Paho), ohne fachliche Topics.

## Services

| Service                                    | Aufgabe                                                   |
|--------------------------------------------|-----------------------------------------------------------|
| [modbus-meters](services/modbus-meters/)   | Zähler per Modbus TCP lesen, Werte über MQTT veröffentlichen |

## Struktur

```
go.work                       verbindet die Module für lokale Entwicklung und die IDE
broker/
  go.mod                      Broker-Modul, versioniert über Tags broker/vX.Y.Z
  broker.go                   Message, Interface MessageHandler
  mqtt/                       MQTT-Implementierung mit Paho
contracts/
  go.mod                      Vertragsmodul, versioniert über Tags contracts/vX.Y.Z
  topic.go                    gemeinsamer Typ contracts.Topic
  meter/                      Topics, MeterID und Payloads rund um Zähler
  modbus/                     Config und Profil eines Modbus-Zählers (nur Daten + Validate)
  physics/                    Formeln zwischen elektrischen Größen
  electricity/                Darstellung elektrischer Größen (Wert, Quelle)
services/
  modbus-meters/
    go.mod                    eigenes Modul
    main.go
    internal/                 nur für diesen Service sichtbar
    Dockerfile
    README.md
.github/workflows/
  broker.yml                  läuft nur bei Änderungen unter broker/
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
go get github.com/KlinkerD/Ems/contracts@v0.4.0
```

```go
import contract "github.com/KlinkerD/Ems/contracts/meter"
```

Lokal hat `go.work` Vorrang: Änderungen in `contracts/` sind sofort in allen
Services sichtbar, ohne neuen Tag. Docker und CI nutzen dagegen die Version aus
dem `go.mod` des Service.

Ablauf bei einer Änderung:

1. `contracts/` ändern und die betroffenen Services im selben Branch anpassen.
2. Nach dem Merge die nächste Version taggen und pushen, z. B. nach v0.4.0:
   `git tag contracts/v0.5.0` und `git push origin contracts/v0.5.0`.
3. In jedem Service, der die Änderung braucht:
   `go get github.com/KlinkerD/Ems/contracts@v0.5.0`.
4. Vor dem Push jeden geänderten Service einmal ohne Workspace bauen, so wie
   Docker es tut: `GOWORK=off go build ./...` im Service-Ordner.

Regeln:

- Felder und Topics nur hinzufügen, nie entfernen oder umbenennen, solange ein
  Service die alte Version nutzt. Neue Felder sind optional (`omitempty`).
- Eine inkompatible Änderung bekommt eine neue Major-Version (`contracts/v2`)
  oder ein neues Topic.
- `contracts` importiert nur die Standardbibliothek.

## Broker-Modul

Einbinden in einem Service:

```powershell
go get github.com/KlinkerD/Ems/broker@v0.3.0
```

```go
import (
	"github.com/KlinkerD/Ems/broker"
	"github.com/KlinkerD/Ems/broker/mqtt"
	contract "github.com/KlinkerD/Ems/contracts/meter"
)

h, err := mqtt.New(mqtt.Config{
	BrokerURL: "tcp://localhost:1883",
	ClientID:  "mein-service",
	Username:  os.Getenv("MQTT_USERNAME"),
	Password:  os.Getenv("MQTT_PASSWORD"),
})
if err != nil { ... }
var msgHandler broker.MessageHandler = h
msgHandler.Connect()
msgHandler.Subscribe(contract.TopicMeterIDs, func(m broker.Message) { ... })
msgHandler.Subscribe(contract.TopicConfig.ForMeter(contract.MustMeterID("umg-1")), func(m broker.Message) { ... })
```

Services importieren Paho nicht selbst, das steckt komplett in `broker/mqtt`.

`broker` kennt keine fachlichen Topics. Es arbeitet nur mit dem gemeinsamen Typ
`contracts.Topic` aus dem Root-Paket von `contracts`:

- `contracts.Topic` ist ein konkretes Topic, z. B. `meter/umg-1/values`.
- `meter.ModbusTopic` ist eine Vorlage mit `{id}`, definiert auf Basis von
  `contracts.Topic`. `ForMeter(id)` liefert daraus ein `contracts.Topic`.
- Topics ohne Platzhalter (z. B. `meter.TopicMeterIDs`) sind direkt ein
  `contracts.Topic`.

Abhängigkeiten laufen nur in eine Richtung: `broker` und `contracts/meter`
nutzen `contracts`, kennen sich aber nicht gegenseitig. Neue Versionen werden
wie beim Vertrag über Tags `broker/vX.Y.Z` veröffentlicht.
