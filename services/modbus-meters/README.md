# modbus-meters

Liest Energiezähler per **Modbus TCP** aus und veröffentlicht Messwerte und
Geräteinfos über **MQTT**. Welche Zähler gelesen werden, kommt zur Laufzeit
über MQTT. Configs können also ohne Neustart hinzugefügt oder ersetzt werden.

## Voraussetzungen

- Go 1.27 oder neuer
- ein MQTT-Broker (z. B. Mosquitto)
- ein Zähler oder Simulator, der per Modbus TCP erreichbar ist

## Starten

```powershell
cd services/modbus-meters
$env:MQTT_USERNAME = "dev"; $env:MQTT_PASSWORD = "devpass"; go run .
```

| Flag         | Standard               | Bedeutung                                     |
|--------------|------------------------|-----------------------------------------------|
| `-broker`    | `tcp://localhost:1883` | Adresse des MQTT-Brokers                      |
| `-client-id` | `modbus-meters`        | MQTT-Client-ID, muss pro Client eindeutig sein |
| `-debug`     | `false`                | jeden Publish loggen                          |

Zugangsdaten zum Broker kommen aus den Umgebungsvariablen `MQTT_USERNAME` und
`MQTT_PASSWORD`, damit sie nicht in der Prozessliste oder Shell-History landen.

## MQTT-Topics

| Topic                | Richtung | Inhalt                                          |
|----------------------|----------|-------------------------------------------------|
| `config`             | rein     | Liste der Meter-IDs, deren Config abonniert wird |
| `meter/{id}/config`  | rein     | Config eines einzelnen Zählers                  |
| `meter/{id}/values`  | raus     | Messwerte, einmal pro Poll-Intervall            |
| `meter/{id}/info`    | raus     | Geräteinfos, bei jedem Verbindungsaufbau        |
| `meter/{id}/status`  | –        | reserviert, noch nicht verwendet                |
| `meter/{id}/error`   | raus     | Erros beim auslesen                             |

Ablauf: Das Programm abonniert `config`. Kommt dort eine Liste von IDs, abonniert
es `meter/{id}/config` für jede ID. Jede gültige Meter-Config startet den Zähler,
eine neue Config mit derselben ID ersetzt die alte. Configs sollten mit dem
Retain-Flag veröffentlicht werden, damit sie nach einem Neustart sofort ankommen.

### `config`

```json
{ "version": 1, "meters": ["umg96rm-1", "umg96rm-2"] }
```

### `meter/{id}/config`

```json
{
  "version": 1,
  "id": "umg96rm-1",
  "host": "192.168.1.50",
  "port": 502,
  "unitId": 1,
  "pollingInterval": "1s",
  "timeout": "500ms",
  "profile": {
    "deviceType": "Janitza UMG 96 RM",
    "meterValues": {
      "U1": { "address": 19000, "length": 2, "registerType": "holding", "dataType": "float32" },
      "ActiveEnergyImported": { "address": 19068, "length": 2, "registerType": "holding", "dataType": "float32" }
    }
  }
}
```

Ein vollständiges Beispiel liegt in
[internal/meter/testdata/umg96rm-1.json](internal/meter/testdata/umg96rm-1.json).

| Feld              | Pflicht | Bedeutung                                              |
|-------------------|---------|--------------------------------------------------------|
| `id`              | ja      | eindeutige ID, wird im Topic verwendet                 |
| `host`            | ja      | Adresse des Geräts                                     |
| `port`            | nein    | Standard `502`                                         |
| `unitId`          | nein    | Modbus Unit-ID, Standard `0`                           |
| `pollingInterval` | ja      | z. B. `"1s"`, `"500ms"`                                |
| `timeout`         | nein    | pro Anfrage, Standard `"1s"`                           |
| `profile`         | ja      | wo welcher Wert im Gerät liegt, siehe unten            |

#### Profil

`meterValues` ordnet einem Messwert-Typ ein Register zu. Erlaubte Typen:
`U1`–`U3` (V), `I1`–`I3` (A), `P1`–`P3` (W), `Q1`–`Q3` (var), `S1`–`S3` (VA),
`ActiveEnergyImported`, `ActiveEnergyExported` (Wh).

`deviceInfo` ist optional und kennt die Felder `serialNumber`, `manufacturer`,
`model` und `firmware`. Sie werden als Text dekodiert.

| Feld           | Bedeutung                                                                 |
|----------------|---------------------------------------------------------------------------|
| `address`      | Startadresse des Registers                                                |
| `length`       | Anzahl 16-Bit-Register, muss zu `dataType` passen                         |
| `registerType` | `holding` (FC 0x03, Standard) oder `input` (FC 0x04)                      |
| `byteOrder`    | `ABCD` (Standard), `DCBA`, `BADC`, `CDAB`                                 |
| `dataType`     | `int16`, `uint16`, `int32`, `uint32`, `int64`, `uint64`, `float32`, `float64` |
| `scale`        | Faktor für den Rohwert, z. B. `1000` für kWh → Wh                          |

Direkt aneinanderliegende Register gleichen Typs werden in einer Anfrage gelesen.

### `meter/{id}/values`

```json
[
  { "meterId": "umg96rm-1", "timestamp": "2026-10-20T22:04:07.129+02:00", "type": "U1", "value": 230.5 }
]
```

### `meter/{id}/info`

```json
{ "meterId": "umg96rm-1", "serialNumber": "...", "manufacturer": "..." }
```

## Projektstruktur

```
main.go                 Einstieg: Flags, MQTT-Verbindung, Weiterleitung der Werte
internal/
  broker/               Topics, Nachrichten, Interface MessageHandler
  broker/mqtt/          MQTT-Implementierung mit Paho
  measurement/          Datentypen: Value, DeviceInfo, Type, Unit
  modbustcp/            minimaler Modbus-TCP-Client (nur Register lesen)
  profile/              Registerbeschreibung und Dekodierung
  meter/                Meter-Config laden/prüfen, Werte und Infos lesen
  poller/               Meter zur Laufzeit starten, ersetzen, stoppen, neu verbinden
```

Abhängigkeiten laufen nur in eine Richtung: `poller` kennt nur `measurement`,
`meter` baut auf `profile` und `modbustcp` auf.

## Entwicklung

```powershell
go test ./...
go vet ./...
```

Die Tests brauchen weder Broker noch Gerät: `meter` nutzt einen Fake-Modbus-Server,
`poller` ein Fake-Meter.
