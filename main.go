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

	paho "github.com/eclipse/paho.mqtt.golang"

	"modbus/internal/broker"
	"modbus/internal/broker/mqtt"
	"modbus/internal/measurement"
	"modbus/internal/meter"
	"modbus/internal/poller"
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

	valuesCh := make(chan []measurement.Value)
	infoCh := make(chan measurement.DeviceInfo)
	manager := poller.NewManager(ctx, valuesCh, infoCh)

	opts := paho.NewClientOptions().
		AddBroker(*brokerURL).
		SetClientID(*clientID).
		SetUsername(os.Getenv("MQTT_USERNAME")).
		SetPassword(os.Getenv("MQTT_PASSWORD"))

	var msgHandler broker.MessageHandler = mqtt.New(opts)
	if err := msgHandler.Connect(); err != nil {
		slog.Error("verbindung zum broker fehlgeschlagen", "broker", *brokerURL, "err", err)
		os.Exit(1)
	}
	defer msgHandler.Close()

	// Muss laufen, bevor Meter starten, sonst blockieren sie beim ersten Senden.
	go forward(ctx, msgHandler, valuesCh, infoCh)

	err := msgHandler.Subscribe(broker.TopicConfig, func(m broker.Message) {
		handleConfig(m, msgHandler, manager)
	})
	if err != nil {
		slog.Error("abonnieren fehlgeschlagen", "topic", broker.TopicConfig, "err", err)
		os.Exit(1)
	}

	<-ctx.Done()   // warten auf Strg+C
	manager.Wait() // warten, bis alle Meter gestoppt sind
}

// forward gibt Messwerte und Geräteinfos aus den Channels an den Broker
// weiter, bis ctx abgebrochen wird.
func forward(ctx context.Context, msgHandler broker.MessageHandler, valuesCh <-chan []measurement.Value, infoCh <-chan measurement.DeviceInfo) {
	for {
		select {
		case vals := <-valuesCh:
			if len(vals) == 0 {
				continue
			}
			publishJSON(msgHandler, broker.TopicMeterValues.ForMeter(vals[0].MeterID), vals)
		case info := <-infoCh:
			publishJSON(msgHandler, broker.TopicMeterInfo.ForMeter(info.MeterID), info)
		case <-ctx.Done():
			return
		}
	}
}

// handleConfig abonniert die Einzel-Config jedes Meters aus der Liste.
func handleConfig(m broker.Message, msgHandler broker.MessageHandler, manager *poller.Manager) {
	slog.Info("config empfangen", "topic", m.Topic)
	var cfg broker.MeterIDs
	if err := json.Unmarshal(m.Payload, &cfg); err != nil {
		slog.Error("ungültige config", "topic", m.Topic, "err", err)
		return
	}
	for _, id := range cfg.Meters {
		topic := broker.TopicMeterConfig.ForMeter(id)
		err := msgHandler.Subscribe(topic, func(m broker.Message) {
			handleMeterConfig(m, manager)
		})
		if err != nil {
			slog.Error("abonnieren fehlgeschlagen", "topic", topic, "err", err)
		}
	}
}

// handleMeterConfig startet den Meter oder ersetzt seine bisherige Config.
func handleMeterConfig(m broker.Message, manager *poller.Manager) {
	slog.Info("meter-config empfangen", "topic", m.Topic)
	mm, err := meter.Parse(m.Payload)
	if err != nil {
		slog.Error("ungültige meter-config", "topic", m.Topic, "err", err)
		return
	}
	if err := manager.Set(mm); err != nil {
		slog.Error("meter starten fehlgeschlagen", "meter", mm.ID, "err", err)
		return
	}
	slog.Info("meter gestartet", "meter", mm.ID, "host", mm.Host, "interval", mm.PollingInterval)
}

// publishJSON serialisiert payload und veröffentlicht es auf topic. Fehler
// werden geloggt, damit ein einzelner fehlgeschlagener Publish nicht das
// Programm beendet.
func publishJSON(msgHandler broker.MessageHandler, topic broker.Topic, payload any) {
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
