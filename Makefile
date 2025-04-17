send:
	GOARCH=arm64 go build .
	scp licznik rpizero:/opt/licznik/

run:
	ssh rpizero "/opt/licznik/licznik"

all:
	GOARCH=arm64 go build .
	scp golicznik rpizero:
	ssh rpizero "/opt/licznik/licznik"

.PHONY: build run all