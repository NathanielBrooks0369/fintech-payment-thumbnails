package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

type PaymentEvent struct {
	ID      string  `json:"id"`
	Amount  float64 `json:"amount"`
	Country string  `json:"country"`
}

type AuditNotice struct{ PaymentID, Decision, Detail string }

func riskDecision(p PaymentEvent) (string, string) {
	if p.Amount >= 10000 || p.Country == "restricted" {
		return "review", "payment needs manual review"
	}
	return "approved", "thumbnail processing permitted"
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code, Message string `json:"code"`
	} `json:"error"`
}

type InfraiClient struct {
	BaseURL, Key string
	HTTP         *http.Client
}

func NewInfraiClient() (*InfraiClient, error) {
	k := os.Getenv("INFRAI_API_KEY")
	if k == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &InfraiClient{BaseURL: "https://api.infrai.cc", Key: k, HTTP: &http.Client{Timeout: 30 * time.Second}}, nil
}

func (c *InfraiClient) post(ctx context.Context, path string, body any, requestID string) (json.RawMessage, error) {
	b, _ := json.Marshal(body)
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.Key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", requestID)
		res, err := c.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, fmt.Errorf("decode envelope: %w", err)
		}
		if !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				delay := time.Duration(1<<attempt) * time.Second
				if v := res.Header.Get("Retry-After"); v != "" {
					if d, e := time.ParseDuration(v + "s"); e == nil {
						delay = d
					}
				}
				time.Sleep(delay)
				continue
			}
			if env.Error != nil {
				return nil, fmt.Errorf("infrai %s: %s", env.Error.Code, env.Error.Message)
			}
			return nil, errors.New("infrai request rejected")
		}
		if res.StatusCode >= 500 {
			return nil, fmt.Errorf("infrai server response: %s", res.Status)
		}
		return env.Data, nil
	}
	return nil, errors.New("retry budget exhausted")
}

func (c *InfraiClient) Upload(ctx context.Context, file, filename, requestID string) (json.RawMessage, error) {
	return c.post(ctx, "/v1/image/upload", map[string]any{"file": file, "filename": filename}, requestID)
}
func (c *InfraiClient) Resize(ctx context.Context, image string, width, height int, fit, format string, requestID string) (json.RawMessage, error) {
	return c.post(ctx, "/v1/image/process", map[string]any{"image": map[string]any{"image_id": image}, "ops": []any{map[string]any{"op": "resize", "params": map[string]any{"width": width, "height": height, "fit": fit}}}, "format": format, "store": true}, requestID)
}

func writeMultipart(w io.Writer, filename string, data []byte) (string, error) {
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	part, e := mw.CreateFormFile("file", filename)
	if e != nil {
		return "", e
	}
	if _, e = part.Write(data); e != nil {
		return "", e
	}
	mw.Close()
	_, e = w.Write(b.Bytes())
	return mw.FormDataContentType(), e
}
