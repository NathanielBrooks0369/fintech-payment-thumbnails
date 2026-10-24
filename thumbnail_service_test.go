package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResizeRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"width", "height", "fit", "enlarge"} {
			if _, ok := body[field]; ok {
				t.Errorf("unexpected top-level field %q", field)
			}
		}
		var ops []struct {
			Op     string `json:"op"`
			Params struct {
				Width  int    `json:"width"`
				Height int    `json:"height"`
				Fit    string `json:"fit"`
			} `json:"params"`
		}
		if err := json.Unmarshal(body["ops"], &ops); err != nil || len(ops) != 1 || ops[0].Op != "resize" || ops[0].Params.Width != 320 || ops[0].Params.Height != 180 || ops[0].Params.Fit != "cover" {
			t.Errorf("invalid resize ops: %s (%v)", body["ops"], err)
		}
		var image map[string]string
		if err := json.Unmarshal(body["image"], &image); err != nil || image["image_id"] != "source" {
			t.Errorf("invalid image reference: %s (%v)", body["image"], err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"data":{}}`))
	}))
	defer server.Close()
	c := &InfraiClient{BaseURL: server.URL, HTTP: server.Client()}
	if _, err := c.Resize(context.Background(), "source", 320, 180, "cover", "webp", "request"); err != nil {
		t.Fatal(err)
	}
}

func TestRiskDecision(t *testing.T) {
	tests := []struct {
		name string
		p    PaymentEvent
		want string
	}{{"small domestic", PaymentEvent{ID: "p1", Amount: 25, Country: "US"}, "approved"}, {"large payment", PaymentEvent{ID: "p2", Amount: 10000, Country: "US"}, "review"}, {"restricted country", PaymentEvent{ID: "p3", Amount: 20, Country: "restricted"}, "review"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := riskDecision(tt.p)
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}
