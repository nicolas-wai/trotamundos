# Makefile del prototipo Trotamundos
.PHONY: run test build vet clean

run:
	go run ./cmd/trotamundos

test:
	go test ./...

build:
	go build -o bin/trotamundos ./cmd/trotamundos

vet:
	go vet ./...

# Borra los archivos SQLite locales y los binarios
clean:
	rm -f data/*.db
	rm -rf bin
