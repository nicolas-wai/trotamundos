// Package config contiene la configuración de una región completa.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// MaxPollInterval es el sondeo máximo permitido para cumplir QA01.
const MaxPollInterval = 2 * time.Minute

// RegionConfig parametriza una región completa del sistema.
// [QA-13] Multi-región: una región se describe solo por configuración, sin tocar la lógica central.
// [ADR-21] Sistema completo replicado por región. [ADR-22] Parámetros de heartbeat. [ADR-16] TTL del access token.
type RegionConfig struct {
	RegionID               string
	LoadBalancerPort       int           // Puerto del balanceador (entrada de la región)
	ReplicaCount           int           // Cantidad de réplicas del monolito [ADR-08]
	ReplicaBasePort        int           // Puerto de la primera réplica; las demás son consecutivas
	ShardCount             int           // Cantidad de shards por idViajero [ADR-11]
	DataDir                string        // Directorio de los archivos .db
	PollInterval           time.Duration // Intervalo de sondeo de proveedores, <= 2 min [ADR-05]
	HeartbeatInterval      time.Duration // Pulso de heartbeat, 10 s [ADR-22]
	HeartbeatMissThreshold int           // Pulsos perdidos para considerar caído un nodo, 3 [ADR-22]
	AccessTokenTTL         time.Duration // Vida del access token, 900 s [ADR-16]
	JWTSecret              string        // [Prototipo] secreto HS256 de desarrollo
}

// Default devuelve la configuración por defecto del prototipo.
func Default() RegionConfig {
	return RegionConfig{
		RegionID:               "region-1",
		LoadBalancerPort:       8080,
		ReplicaCount:           2,
		ReplicaBasePort:        8081,
		ShardCount:             2,
		DataDir:                "./data",
		PollInterval:           MaxPollInterval,
		HeartbeatInterval:      10 * time.Second,
		HeartbeatMissThreshold: 3,
		AccessTokenTTL:         900 * time.Second,
		JWTSecret:              "trotamundos-dev-secret",
	}
}

// ReplicaPorts devuelve los puertos de las réplicas.
func (c RegionConfig) ReplicaPorts() []int {
	ports := make([]int, c.ReplicaCount)
	for i := range ports {
		ports[i] = c.ReplicaBasePort + i
	}
	return ports
}

// Load lee la configuración desde variables de entorno TROTAMUNDOS_* sobre los defaults.
func Load() (RegionConfig, error) {
	return LoadFrom(os.Getenv)
}

// LoadFrom permite inyectar el origen de variables (útil en tests).
func LoadFrom(getenv func(string) string) (RegionConfig, error) {
	c := Default()
	var firstErr error
	setErr := func(key string, err error) {
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("variable %s inválida: %w", key, err)
		}
	}
	str := func(key string, dst *string) {
		if v := getenv(key); v != "" {
			*dst = v
		}
	}
	num := func(key string, dst *int) {
		if v := getenv(key); v != "" {
			n, err := strconv.Atoi(v)
			setErr(key, err)
			if err == nil {
				*dst = n
			}
		}
	}
	dur := func(key string, dst *time.Duration) {
		if v := getenv(key); v != "" {
			d, err := time.ParseDuration(v)
			setErr(key, err)
			if err == nil {
				*dst = d
			}
		}
	}

	str("TROTAMUNDOS_REGION_ID", &c.RegionID)
	num("TROTAMUNDOS_LB_PORT", &c.LoadBalancerPort)
	num("TROTAMUNDOS_REPLICA_COUNT", &c.ReplicaCount)
	num("TROTAMUNDOS_REPLICA_BASE_PORT", &c.ReplicaBasePort)
	num("TROTAMUNDOS_SHARD_COUNT", &c.ShardCount)
	str("TROTAMUNDOS_DATA_DIR", &c.DataDir)
	dur("TROTAMUNDOS_POLL_INTERVAL", &c.PollInterval)
	dur("TROTAMUNDOS_HEARTBEAT_INTERVAL", &c.HeartbeatInterval)
	num("TROTAMUNDOS_HEARTBEAT_MISS_THRESHOLD", &c.HeartbeatMissThreshold)
	dur("TROTAMUNDOS_ACCESS_TOKEN_TTL", &c.AccessTokenTTL)
	str("TROTAMUNDOS_JWT_SECRET", &c.JWTSecret)

	if firstErr != nil {
		return c, firstErr
	}
	return c, c.Validate()
}

// Validate verifica los invariantes de la configuración.
func (c RegionConfig) Validate() error {
	switch {
	case c.PollInterval <= 0 || c.PollInterval > MaxPollInterval:
		return fmt.Errorf("el intervalo de sondeo debe estar entre 0 y %s (QA01)", MaxPollInterval)
	case c.ReplicaCount < 1:
		return fmt.Errorf("se requiere al menos una réplica")
	case c.ShardCount < 1:
		return fmt.Errorf("se requiere al menos un shard")
	case c.HeartbeatInterval <= 0 || c.HeartbeatMissThreshold < 1:
		return fmt.Errorf("parámetros de heartbeat inválidos")
	case c.AccessTokenTTL <= 0:
		return fmt.Errorf("el TTL del access token debe ser positivo")
	}
	return nil
}
