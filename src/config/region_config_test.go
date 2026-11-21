package config

import (
	"testing"
	"time"
)

func TestDefaultsMatchArchitecture(t *testing.T) {
	c := Default()
	if c.PollInterval > 2*time.Minute || c.HeartbeatInterval != 10*time.Second ||
		c.HeartbeatMissThreshold != 3 || c.AccessTokenTTL != 900*time.Second {
		t.Fatalf("defaults no respetan el AD: %+v", c)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := c.ReplicaPorts(); len(got) != 2 || got[0] != 8081 || got[1] != 8082 {
		t.Fatalf("puertos de réplicas inesperados: %v", got)
	}
}

func TestLoadFromOverrides(t *testing.T) {
	env := map[string]string{
		"TROTAMUNDOS_REGION_ID":     "region-x",
		"TROTAMUNDOS_REPLICA_COUNT": "3",
		"TROTAMUNDOS_POLL_INTERVAL": "30s",
	}
	c, err := LoadFrom(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if c.RegionID != "region-x" || c.ReplicaCount != 3 || c.PollInterval != 30*time.Second {
		t.Fatalf("overrides no aplicados: %+v", c)
	}
}

func TestLoadFromRejectsPollAboveTwoMinutes(t *testing.T) {
	_, err := LoadFrom(func(k string) string {
		if k == "TROTAMUNDOS_POLL_INTERVAL" {
			return "3m"
		}
		return ""
	})
	if err == nil {
		t.Fatal("se esperaba error con sondeo > 2 min")
	}
}

func TestLoadFromRejectsMalformedNumber(t *testing.T) {
	_, err := LoadFrom(func(k string) string {
		if k == "TROTAMUNDOS_SHARD_COUNT" {
			return "abc"
		}
		return ""
	})
	if err == nil {
		t.Fatal("se esperaba error con número inválido")
	}
}

func TestLoadFromProviderSettings(t *testing.T) {
	env := map[string]string{
		"TROTAMUNDOS_POLLING": "true", "TROTAMUNDOS_PROVIDER_RETRIES": "5", "TROTAMUNDOS_PROVIDER_BACKOFF": "2s",
		"TROTAMUNDOS_PROVIDER_COOLDOWN": "45s", "TROTAMUNDOS_TOKEN_KEY": "otra-clave",
	}
	c, err := LoadFrom(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if !c.PollingEnabled || c.ProviderMaxRetries != 5 || c.ProviderBackoff != 2*time.Second || c.ProviderCooldown != 45*time.Second || c.TokenEncryptionKey != "otra-clave" {
		t.Fatalf("%+v", c)
	}
	d := Default()
	if d.PollingEnabled || d.ProviderMaxRetries != 3 || d.TokenEncryptionKey == "" {
		t.Fatalf("defaults: %+v", d)
	}
	if _, err := LoadFrom(func(k string) string {
		if k == "TROTAMUNDOS_POLLING" {
			return "quizas"
		}
		return ""
	}); err == nil {
		t.Fatal("booleano inválido")
	}
	if _, err := LoadFrom(func(k string) string {
		if k == "TROTAMUNDOS_PROVIDER_COOLDOWN" {
			return "0s"
		}
		return ""
	}); err == nil {
		t.Fatal("cooldown cero")
	}
}
