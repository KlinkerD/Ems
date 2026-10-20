package main

import (
	"context"
	"encoding/json"
	"log"
	"modbus/broker"
	"modbus/broker/mqtt"
	"modbus/configs"
	"modbus/meters"
	"modbus/meters/values"
	"os"
	"os/signal"

	paho "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)

	defer stop()

	valuesCh := make(chan []values.MeterValue)
	infoCh := make(chan values.DeviceInfo)

	modbusManager := meters.NewManager(ctx, valuesCh, infoCh)

	// 1. Optionen für paho bauen
	opts := paho.NewClientOptions().
		AddBroker("tcp://localhost:1883"). // Adresse deines Brokers
		SetUsername("dev").
		SetPassword("devpass").
		SetClientID("modbus-meters") // muss pro Client eindeutig sein
	// optional: opts.SetUsername("user").SetPassword("pass")

	// 2. Deinen Handler erzeugen
	var msgHandler broker.MessageHandler = mqtt.New(opts)

	// 3. Verbinden
	if err := msgHandler.Connect(); err != nil {
		log.Fatal(err)
	}
	defer msgHandler.Close()

	// 4. Channels auslesen und an MQTT weitergeben. Muss laufen, bevor Meter
	// starten, sonst blockieren sie beim ersten Senden.
	go func() {
		for {
			select {
			case vals := <-valuesCh:
				if len(vals) == 0 {
					continue
				}
				publishJSON(msgHandler, broker.METER_VALUES.ForMeter(vals[0].MeterID), vals)
			case info := <-infoCh:
				publishJSON(msgHandler, broker.METER_INFO.ForMeter(info.MeterID), info)
			case <-ctx.Done():
				return
			}
		}
	}()

	// 5. Abonnieren
	err := msgHandler.Subscribe(broker.CONFIG, func(m broker.Message) {
		handleNewConfig(m, msgHandler, modbusManager)
	})
	if err != nil {
		log.Fatal(err)
	}

	<-ctx.Done()         // warten auf Strg+C
	modbusManager.Wait() // warten, bis alle Meter gestoppt sind
}

func handleNewConfig(m broker.Message, msgHandler broker.MessageHandler, modbusManager *meters.Manager) {
	log.Println("New Config Received {}", m)
	var cfg configs.MeterIds
	if err := json.Unmarshal(m.Payload, &cfg); err != nil {
		log.Printf("ungültige config auf %s: %v", m.Topic, err)
		return
	}
	for _, meter := range cfg.Meters {
		var topic = broker.Topic.ForMeter(broker.METER_CONFIG, meter)
		err := msgHandler.Subscribe(topic, func(m broker.Message) {
			handleNewMeterConfig(m, modbusManager)
		})
		if err != nil {
			log.Fatal("Could not Subscribe to {}", topic)
		}
	}
}

func handleNewMeterConfig(m broker.Message, modbusManager *meters.Manager) {
	log.Println("New Meter Config Received {}", m)
	meter, err := meters.ParseMeter(m.Payload)
	if err != nil {
		log.Fatal(err)
	}
	err = modbusManager.Set(meter)
	if err != nil {
		log.Fatal(err)
	}
}

func publishJSON(msgHandler broker.MessageHandler, topic broker.Topic, payload any) {
	log.Println("Publishing JSON {}", topic)
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Fatal(err)
	}
	err = msgHandler.Publish(broker.Message{Topic: topic, Payload: jsonPayload})
	if err != nil {
		log.Fatal(err)
	}
}
