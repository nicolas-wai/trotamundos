# Makefile del prototipo Trotamundos
.PHONY: run test test-e2e e2e-install build vet clean

run:
	go run ./cmd/trotamundos

test:
	go test ./...

# Pruebas de navegador real (Playwright + Chromium headless). Requieren `make e2e-install` una vez.
test-e2e:
	go test -tags e2e ./src/client/web -count=1 -v

e2e-install:
	go run github.com/mxschmitt/playwright-go/cmd/playwright install chromium

build:
	go build -o bin/trotamundos ./cmd/trotamundos

vet:
	go vet ./...

# Borra los archivos SQLite locales y los binarios
clean:
	rm -f data/*.db
	rm -rf data/*/
	rm -rf bin
