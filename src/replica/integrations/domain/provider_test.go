package domain

import "testing"

func TestProviderIDConvention(t *testing.T) {
	id := ProviderIDFor(ProviderAgency, " Agencia-Sol ")
	if id != "agencia:agencia-sol" || ProviderTypeOf(id) != ProviderAgency {
		t.Fatalf("%q %q", id, ProviderTypeOf(id))
	}
	if ProviderTypeOf(ProviderIDFor(ProviderEmail, "gmail")) != ProviderEmail || ProviderTypeOf(DefaultTravelSystemProviderID) != ProviderTravelSystem {
		t.Fatal("tipo deducido del id")
	}
	if ProviderTypeOf("raro") != "" {
		t.Fatal("un id fuera de convención no tiene tipo")
	}
}
