package main

import (
	"bufio"
	"fmt"
	"log"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.bug.st/serial"
)

const (
	tty               = "/dev/ttyUSB0"
	mqttTopic         = "house/energy/total"
	mqttStatusTopic   = "house/energy/status"
	mqttBrokerAddress = "ip://192.168.0.5:8883"
	expectedMeterID   = "/ZPA4ZE314"
)

func main() {
	mqttClient := mqtt.NewClient(mqtt.NewClientOptions().
		AddBroker(mqttBrokerAddress).
		SetClientID("mqtt-client").
		SetCleanSession(true).
		SetKeepAlive(60))
	if mqttClient == nil {
		log.Fatal("Failed to create MQTT client")
	}

	// mqttClient.Publish(mqttStatusTopic, []byte("{\"status\":\"Odczyt\"}"))

	port, err := serial.Open(tty, &serial.Mode{
		BaudRate: 300,
		DataBits: 7,
		StopBits: serial.OneStopBit,
		Parity:   serial.EvenParity,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer port.Close()

	port.SetReadTimeout(time.Second * 5)

	reader := bufio.NewReader(port)

	//wake up
	wakeupSeq := [64]byte{}
	port.Write(wakeupSeq[:])

	//sign on
	port.Write([]byte("/?!\r\n"))
	response, err := reader.ReadBytes('\n')
	if err != nil {
		log.Fatal(err)
	}
	if !strings.HasPrefix(string(response), expectedMeterID) {
		log.Fatalf("unexpected ID string: %s", response)
	}

	time.Sleep(time.Millisecond * 300)

	//confirm new speed
	speedConfirmation := "\x06040\r\n"
	port.Write([]byte(speedConfirmation))

	time.Sleep(time.Millisecond * 300)

	port.SetMode(&serial.Mode{
		BaudRate: 4800,
		DataBits: 7,
		StopBits: serial.OneStopBit,
		Parity:   serial.EvenParity,
	})

	// read response
	for {
		response, err = reader.ReadBytes('\n')
		if err != nil {
			log.Fatalf("response: %s, err %v\n", response, err)
		}
		fmt.Printf("Response: %s", response)
		if response[0] == '!' {
			break
		}
	}

	// mqttClient.Publish(mqttStatusTopic, []byte("{\"status\":\"OK\"}"))
}
