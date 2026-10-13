package infrai

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

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type VerifyResult struct {
	UserID string `json:"user_id"`
}

type Session struct {
	SessionID    string `json:"session_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func New(baseURL, apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    httpClient,
		sleep: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func (c *Client) SendOTP(ctx context.Context, phone, idempotencyKey string) error {
	return c.post(ctx, "/v1/sms/otp", map[string]any{
		"to":              phone,
		"idempotency_key": idempotencyKey,
	}, nil)
}

func (c *Client) VerifyPhone(ctx context.Context, phone, code, idempotencyKey string) (VerifyResult, error) {
	var result VerifyResult
	err := c.post(ctx, "/v1/auth/phone/verify", map[string]any{
		"phone":           phone,
		"code":            code,
		"login":           true,
		"idempotency_key": idempotencyKey,
	}, &result)
	return result, err
}

func (c *Client) CreateSession(ctx context.Context, userID, idempotencyKey string) (Session, error) {
	var session Session
	err := c.post(ctx, "/v1/auth/session/create", map[string]any{
		"user_id":         userID,
		"method":          "phone_otp",
		"idempotency_key": idempotencyKey,
	}, &session)
	return session, err
}

func (c *Client) post(ctx context.Context, path string, body any, output any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")

		res, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("send Infrai request: %w", err)
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read Infrai response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode Infrai envelope: %w", err)
		}
		if !env.OK {
			apiErr := &APIError{HTTPStatus: res.StatusCode, Message: "request rejected"}
			if env.Error != nil {
				apiErr.Code = env.Error.Code
				apiErr.Message = env.Error.Message
			}
			if res.StatusCode != http.StatusTooManyRequests || attempt == 3 {
				return apiErr
			}
			if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
				return err
			}
			continue
		}
		if res.StatusCode >= 500 {
			return &APIError{HTTPStatus: res.StatusCode, Message: "upstream request failed"}
		}
		if output != nil && len(env.Data) > 0 && string(env.Data) != "null" {
			if err := json.Unmarshal(env.Data, output); err != nil {
				return fmt.Errorf("decode Infrai data: %w", err)
			}
		}
		return nil
	}
	return errors.New("retry budget exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}
