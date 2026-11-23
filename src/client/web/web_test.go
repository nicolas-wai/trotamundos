package web

import "testing"

func TestEmbeddedAssets(t *testing.T) {
	if _, err := Templates.ReadFile("templates/dashboard.html"); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"runtime.js", "optimistic_state.js", "visual_preferences.js", "recommendations_panel.js", "itineraries_panel.js"} {
		if _, err := Static.ReadFile("static/" + f); err != nil {
			t.Fatalf("falta static/%s: %v", f, err)
		}
	}
}
