# Makefile del prototipo Trotamundos
.PHONY: run test test-load test-e2e e2e-install build vet clean

run:
	go run ./cmd/trotamundos

test:
	go test ./...

# Prueba de carga [ADR-08]: 500 req/s sostenidos contra GET /dashboard por https (lenta, depende de la máquina;
# no forma parte de `make test`). Deja el resumen en bin/load-summary.txt. Variables: LOAD_RATE, LOAD_DURATION,
# LOAD_USERS, LOAD_WRITE_PCT, LOAD_P95_MS, LOAD_CPUPROFILE.
test-load:
	go test -tags load ./src/region -run TestLoad500RequestsPerSecond -count=1 -v -timeout 10m

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
