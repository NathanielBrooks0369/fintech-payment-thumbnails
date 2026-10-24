package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type thumbnailRequest struct {
	Payment  PaymentEvent `json:"payment"`
	Image    string       `json:"image"`
	Filename string       `json:"filename"`
	Width    int          `json:"width"`
	Height   int          `json:"height"`
}
type thumbnailResponse struct {
	PaymentID, Decision string
	Notice              AuditNotice
	Image               json.RawMessage `json:"image,omitempty"`
}

func thumbnailHandler(client *InfraiClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		var in thumbnailRequest
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid json", 400)
			return
		}
		decision, detail := riskDecision(in.Payment)
		out := thumbnailResponse{PaymentID: in.Payment.ID, Decision: decision, Notice: AuditNotice{in.Payment.ID, decision, detail}}
		if decision == "approved" {
			uploaded, err := client.Upload(context.Background(), in.Image, in.Filename, in.Payment.ID)
			if err != nil {
				http.Error(w, err.Error(), 502)
				return
			}
			var u struct {
				ID string `json:"image_id"`
			}
			json.Unmarshal(uploaded, &u)
			resized, err := client.Resize(context.Background(), u.ID, in.Width, in.Height, "cover", "webp", in.Payment.ID)
			if err != nil {
				http.Error(w, err.Error(), 502)
				return
			}
			out.Image = resized
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}
}

func main() {
	c, err := NewInfraiClient()
	if err != nil {
		fmt.Println(err)
		return
	}
	http.HandleFunc("/thumbnail", thumbnailHandler(c))
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") })
	http.ListenAndServe(":8080", nil)
}
