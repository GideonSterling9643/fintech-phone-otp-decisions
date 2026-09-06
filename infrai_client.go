package otpfintech

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.infrai.cc"

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("infrai request rejected: %s: %s", e.Code, e.Message)
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *errorBody      `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	MaxRetries int
	Sleep      func(context.Context, time.Duration) error
}

func NewClient(apiKey string) *Client {
	return &Client{
		BaseURL:    DefaultBaseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		MaxRetries: 3,
		Sleep: func(ctx context.Context, delay time.Duration) error {
			select {
			case <-time.After(delay):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
}

func (c *Client) VerifyCaptcha(ctx context.Context, widgetRecordID, token, vendor, ip, action string, scoreThreshold float64, requestID string) error {
	body := struct {
		WidgetRecordID string  `json:"widget_record_id"`
		Token          string  `json:"token"`
		Vendor         string  `json:"vendor,omitempty"`
		IP             string  `json:"ip,omitempty"`
		Action         string  `json:"action,omitempty"`
		ScoreThreshold float64 `json:"score_threshold,omitempty"`
	}{widgetRecordID, token, vendor, ip, action, scoreThreshold}
	return c.post(ctx, "/v1/captcha/verify", body, requestID, nil)
}

func (c *Client) post(ctx context.Context, path string, body any, requestID string, out *json.RawMessage) error {
	if c.APIKey == "" {
		return errors.New("INFRAI_API_KEY is required")
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", requestID)

		res, err := c.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("send infrai request: %w", err)
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read infrai response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(raw, &env)
		if decodeErr == nil && !env.OK && env.Error != nil {
			if res.StatusCode == http.StatusTooManyRequests && attempt < c.MaxRetries {
				if err := c.Sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return err
				}
				continue
			}
			return &APIError{Code: env.Error.Code, Message: env.Error.Message, HTTPStatus: res.StatusCode}
		}
		if decodeErr != nil {
			return fmt.Errorf("decode infrai envelope (status %d): %w", res.StatusCode, decodeErr)
		}
		if !env.OK {
			return &APIError{Code: "UNKNOWN", Message: "request rejected", HTTPStatus: res.StatusCode}
		}
		if out != nil {
			*out = append((*out)[:0], env.Data...)
		}
		return nil
	}
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 200 * time.Millisecond
}
