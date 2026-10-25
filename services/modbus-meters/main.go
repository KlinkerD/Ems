// Command modbus liest Zähler per Modbus TCP und veröffentlicht Messwerte und
// Geräteinfos über MQTT. Die Meter-Configs kommen zur Laufzeit über MQTT.
//
// Broker-Adresse und Client-ID werden per Flag gesetzt, Zugangsdaten über die
// Umgebungsvariablen MQTT_USERNAME und MQTT_PASSWORD.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/KlinkerD/Ems/broker"
	"github.com/KlinkerD/Ems/broker/mqtt"
	"github.com/KlinkerD/Ems/contracts"
	contract "github.com/KlinkerD/Ems/contracts/meter"
	"github.com/KlinkerD/Ems/contracts/modbus"
	"github.com/KlinkerD/Ems/services/modbus-meters/internal/meter"
	"github.com/KlinkerD/Ems/services/modbus-meters/internal/poller"
)

func main() {
	brokerURL := flag.String("broker", "tcp://localhost:1883", "Adresse des MQTT-Brokers")
	clientID := flag.String("client-id", "modbus-meters", "MQTT-Client-ID, muss pro Client eindeutig sein")
	debug := flag.Bool("debug", false, "jeden Publish loggen")
	flag.Parse()

	if *debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	valuesCh := make(chan []contract.Value)
	infoCh := make(chan contract.DeviceInfo)
	errCh := make(chan contract.Error)
	statusCh := make(chan contract.Status)
	manager := poller.NewManager(ctx, valuesCh, infoCh, errCh)

	mqttHandler, err := mqtt.New(mqtt.Config{
		BrokerURL: *brokerURL,
		ClientID:  *clientID,
		Username:  os.Getenv("MQTT_USERNAME"),
		Password:  os.Getenv("MQTT_PASSWORD"),
	})
	if err != nil {
		slog.Error("ungültige mqtt-config", "err", err)
		os.Exit(1)
	}
	var msgHandler broker.MessageHandler = mqttHandler
	if err := msgHandler.Connect(); err != nil {
		slog.Error("verbindung zum broker fehlgeschlagen", "broker", *brokerURL, "err", err)
		os.Exit(1)
	}
	defer msgHandler.Close()

	// Muss laufen, bevor Meter starten, sonst blockieren sie beim ersten Senden.
	go forward(ctx, msgHandler, valuesCh, infoCh, errCh, statusCh)

	err = msgHandler.Subscribe(contract.TopicMeterIDs, func(m broker.Message) {
		handleConfig(ctx, m, msgHandler, manager, statusCh)
	})
	if err != nil {
		slog.Error("abonnieren fehlgeschlagen", "topic", contract.TopicMeterIDs, "err", err)
		os.Exit(1)
	}

	<-ctx.Done()   // warten auf Strg+C
	manager.Wait() // warten, bis alle Meter gestoppt sind
}

// forward gibt Messwerte und Geräteinfos aus den Channels an den Broker
// weiter, bis ctx abgebrochen wird.
func forward(ctx context.Context, msgHandler broker.MessageHandler, valuesCh <-chan []contract.Value, infoCh <-chan contract.DeviceInfo, errCh <-chan contract.Error, statusCh <-chan contract.Status) {
	for {
		select {
		case vals := <-valuesCh:
			if len(vals) == 0 {
				continue
			}
			publishJSON(msgHandler, contract.TopicValues.ForMeter(vals[0].MeterID), vals)
		case info := <-infoCh:
			publishJSON(msgHandler, contract.TopicInfo.ForMeter(info.MeterID), info)
		case err := <-errCh:
			publishJSON(msgHandler, contract.TopicError.ForMeter(err.MeterID), err)
		case status := <-statusCh:
			publishJSON(msgHandler, contract.TopicStatus.ForMeter(status.MeterID), status)
		case <-ctx.Done():
			return
		}
	}
}

// handleConfig abonniert die Einzel-Config jedes Meters aus der Liste.
func handleConfig(ctx context.Context, m broker.Message, msgHandler broker.MessageHandler, manager *poller.Manager, statusCh chan<- contract.Status) {
	now := time.Now()
	slog.Info("config empfangen", "topic", m.Topic)
	var cfg contract.MeterIDs
	if err := json.Unmarshal(m.Payload, &cfg); err != nil {
		slog.Error("ungültige config", "topic", m.Topic, "err", err)
		return
	}
	deletedMeters := manager.DeleteMeters(cfg.Meters)
	for _, id := range cfg.Meters {
		topic := contract.TopicConfig.ForMeter(id)
		err := msgHandler.Subscribe(topic, func(m broker.Message) {
			handleMeterConfig(ctx, m, manager, statusCh)
		})
		if err != nil {
			slog.Error("abonnieren fehlgeschlagen", "topic", topic, "err", err)
		}
	}
	for _, id := range deletedMeters {
		topic := contract.TopicConfig.ForMeter(id)
		err := msgHandler.Unsubscribe(topic)
		if err != nil {
			slog.Error("abmelden fehlgeschlagen", "topic", topic, "err", err)
		}
		sendStatus(ctx, statusCh, contract.Status{MeterID: id, Timestamp: now, Message: "Meter Deleted"})
	}
}

// handleMeterConfig startet den Meter oder ersetzt seine bisherige Config.
func handleMeterConfig(ctx context.Context, m broker.Message, manager *poller.Manager, statusCh chan<- contract.Status) {
	var now = time.Now()
	slog.Info("meter-config empfangen", "topic", m.Topic)
	cfg, err := modbus.Parse(m.Payload)
	if err != nil {
		slog.Error("ungültige meter-config", "topic", m.Topic, "err", err)
		// cfg ist nil, die ID steht nur noch im Topic. Ist sie dort nicht
		// lesbar, gibt es kein sinnvolles Status-Topic ("meter//status"),
		// dann bleibt es beim Log.
		id := meterIDFromConfigTopic(m.Topic)
		if id.IsZero() {
			slog.Warn("status nicht gesendet, meter-id im topic unlesbar", "topic", m.Topic)
			return
		}
		sendStatus(ctx, statusCh, contract.Status{MeterID: id, Timestamp: now, Message: "Invalid Config"})
		return
	}
	sendStatus(ctx, statusCh, contract.Status{MeterID: cfg.ID, Timestamp: now, Message: "New Config Received"})
	if err := manager.Set(meter.NewReader(cfg)); err != nil {
		slog.Error("meter starten fehlgeschlagen", "meter", cfg.ID, "err", err)
		sendStatus(ctx, statusCh, contract.Status{MeterID: cfg.ID, Timestamp: now, Message: "Start Failed"})
		return
	}
	slog.Info("meter gestartet", "meter", cfg.ID, "host", cfg.Host, "interval", cfg.PollingInterval)
	sendStatus(ctx, statusCh, contract.Status{MeterID: cfg.ID, Timestamp: now, Message: "Started"})
}

// sendStatus schickt s an forward, gibt aber auf, wenn ctx abgebrochen ist.
// Die Handler laufen in MQTT-Callbacks; nach dem Beenden liest forward nicht
// mehr, und ein blockierendes Senden würde den Callback für immer aufhalten.
func sendStatus(ctx context.Context, statusCh chan<- contract.Status, s contract.Status) {
	select {
	case statusCh <- s:
	case <-ctx.Done():
	}
}

// meterIDFromConfigTopic liest die Meter-ID aus einem Topic nach der Vorlage
// contract.TopicConfig. Passt das Topic nicht oder ist die ID ungültig, ist
// das Ergebnis der Nullwert.
func meterIDFromConfigTopic(topic contracts.Topic) contract.MeterID {
	prefix, suffix, _ := strings.Cut(string(contract.TopicConfig), "{id}")
	s := string(topic)
	if len(s) <= len(prefix)+len(suffix) || !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return contract.MeterID{}
	}
	id, err := contract.NewMeterID(s[len(prefix) : len(s)-len(suffix)])
	if err != nil {
		return contract.MeterID{}
	}
	return id
}

// publishJSON serialisiert payload und veröffentlicht es auf topic. Fehler
// werden geloggt, damit ein einzelner fehlgeschlagener Publish nicht das
// Programm beendet.
func publishJSON(msgHandler broker.MessageHandler, topic contracts.Topic, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		slog.Error("json kodieren fehlgeschlagen", "topic", topic, "err", err)
		return
	}
	if err := msgHandler.Publish(broker.Message{Topic: topic, Payload: data}); err != nil {
		slog.Error("veröffentlichen fehlgeschlagen", "topic", topic, "err", err)
		return
	}
	slog.Debug("veröffentlicht", "topic", topic)
}
