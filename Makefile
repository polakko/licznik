send:
	GOARCH=arm64 go build .
	scp licznik rpizero:/opt/licznik/

run:
	ssh rpizero "/opt/licznik/licznik"

all: send run

.PHONY: build run