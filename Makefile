.PHONY: run build test vet web zip serve clean

PORT ?= 8080

run:
	go run ./cmd/game

build:
	go build -o dist/shift-command ./cmd/game

test:
	go test ./...

vet:
	go vet ./...

web:
	scripts/build-web.sh

zip:
	scripts/build-web.sh --zip

serve: web
	@echo "Serving http://localhost:$(PORT)"
	python3 -m http.server $(PORT) --directory dist/web

clean:
	rm -rf dist
