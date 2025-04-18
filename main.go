package main

import (
	"bufio"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.bug.st/serial"
)

const (
	tty               = "/dev/ttyUSB0"
	mqttTopic         = "house/energy/total"
	mqttStatusTopic   = "house/energy/status"
	mqttBrokerAddress = "tcp://192.168.0.5:1883"
	expectedMeterID   = "/ZPA4ZE314"
	totalCode         = "1.8.0"
	qos               = 0 // At most once delivery
)

func main() {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(mqttBrokerAddress)
	opts.SetClientID("licznik-mqtt-client")
	opts.SetCleanSession(true)
	opts.SetKeepAlive(60)

	mqttClient := mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Error connecting to MQTT broker: %v", token.Error())
	}
	defer mqttClient.Disconnect(250)

	mqttClient.Publish(mqttStatusTopic, qos, false, "{\"status\":\"Odczyt\"}")

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

	// Flush buffers
	if err := port.ResetInputBuffer(); err != nil {
		log.Printf("Error flushing receive buffer: %v\n", err)
	}

	if err := port.ResetOutputBuffer(); err != nil {
		log.Printf("Error flushing transmit buffer: %v\n", err)
	}

	// Wake up
	wakeupSeq := [8]byte{}
	port.Write(wakeupSeq[:])

	// Sign on
	port.Write([]byte("/?!\r\n"))
	response, err := reader.ReadBytes('\n')
	if err != nil {
		log.Fatal(err)
	}
	if !strings.HasPrefix(string(response), expectedMeterID) {
		mqttClient.Publish(mqttStatusTopic, qos, false, "{\"status\":\"Błąd ID\"}")
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

	found := false
	// read response
	for {
		response, err = reader.ReadBytes('\n')
		if err != nil {
			log.Fatalf("response: %s, err %v\n", response, err)
			mqttClient.Publish(mqttStatusTopic, qos, false, "{\"status\":\"Błąd odczytu\"}")
		}
		if strings.HasPrefix(string(response), totalCode) {
			found = true
			sendResult(mqttClient, string(response))
		}
		if response[0] == '!' {
			break
		}
	}
	if !found {
		log.Println("No total code found")
		mqttClient.Publish(mqttStatusTopic, qos, false, "{\"status\":\"Brak linii\"}")
	}
}

func sendResult(mqttClient mqtt.Client, result string) {
	value, err := extractValue(result)
	if err != nil {
		log.Printf("Error extracting value: %v\n", err)
		mqttClient.Publish(mqttStatusTopic, qos, false, "{\"status\":\"Błąd wartości\"}")
		return
	}

	mqttClient.Publish(mqttTopic, qos, false, fmt.Sprintf("%f", value))
	mqttClient.Publish(mqttStatusTopic, qos, false, "{\"status\":\"OK\"}")
}

func extractValue(s string) (float64, error) {
	re := regexp.MustCompile(`\(([^()\*\n]+?)(?:\*|\))`)
	match := re.FindStringSubmatch(s)
	if len(match) > 1 {
		numberStr := match[1]
		floatVal, err := strconv.ParseFloat(numberStr, 64)
		if err != nil {
			return 0.0, fmt.Errorf("failed to parse '%s' as float64: %w", numberStr, err)
		}
		return floatVal, nil
	}
	return 0.0, fmt.Errorf("no matching pattern found in the string '%s'", s)
}
