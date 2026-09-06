package otpfintech

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

type RiskEvent struct {
	SubjectID         string  `json:"subject_id"`
	EventType         string  `json:"event_type"`
	IP                string  `json:"ip"`
	DeviceFingerprint string  `json:"device_fingerprint"`
	Amount            float64 `json:"amount"`
}

type LoginRequest struct {
	RequestID         string  `json:"request_id"`
	Phone             string  `json:"phone"`
	Code              string  `json:"code"`
	CaptchaToken      string  `json:"captcha_token"`
	WidgetRecordID    string  `json:"widget_record_id"`
	AccountID         string  `json:"account_id"`
	PaymentAmount     float64 `json:"payment_amount"`
	IP                string  `json:"ip"`
	DeviceFingerprint string  `json:"device_fingerprint"`
}

type LoginResult struct {
	RequestID string `json:"request_id"`
	Action    string `json:"action"`
	Reason    string `json:"reason"`
	Phone     string `json:"phone"`
}

type AuditNotification struct {
	RecordedAt string  `json:"recorded_at"`
	RequestID  string  `json:"request_id"`
	AccountID  string  `json:"account_id"`
	EventType  string  `json:"event_type"`
	Amount     float64 `json:"amount"`
	Action     string  `json:"action"`
	Reason     string  `json:"reason"`
}

type Gateway interface {
	VerifyCaptcha(context.Context, string, string, string, string, string, float64, string) error
}

type OTPVerifier interface {
	Verify(phone, code string) bool
}

type StaticOTPVerifier struct {
	Code string
}

func (v StaticOTPVerifier) Verify(_ string, code string) bool {
	return subtle.ConstantTimeCompare([]byte(code), []byte(v.Code)) == 1
}

type PaymentLogin struct {
	Gateway Gateway
	OTP     OTPVerifier
	Audit   io.Writer
	Now     func() time.Time
}

func (p PaymentLogin) Verify(ctx context.Context, in LoginRequest) (LoginResult, error) {
	if err := p.Gateway.VerifyCaptcha(ctx, in.WidgetRecordID, in.CaptchaToken, "", in.IP, "payment_login", 0.7, in.RequestID); err != nil {
		return LoginResult{}, err
	}
	if !p.OTP.Verify(in.Phone, in.Code) {
		return LoginResult{}, &LoginError{Message: "invalid verification code"}
	}
	event := RiskEvent{
		SubjectID: in.AccountID, EventType: "payment_login", IP: in.IP,
		DeviceFingerprint: in.DeviceFingerprint, Amount: in.PaymentAmount,
	}
	action, reason := PaymentAction(in.PaymentAmount, in.DeviceFingerprint)
	result := LoginResult{RequestID: in.RequestID, Action: action, Reason: reason, Phone: in.Phone}
	note := AuditNotification{
		RecordedAt: p.Now().UTC().Format(time.RFC3339), RequestID: in.RequestID,
		AccountID: in.AccountID, EventType: event.EventType, Amount: in.PaymentAmount,
		Action: action, Reason: reason,
	}
	if err := json.NewEncoder(p.Audit).Encode(note); err != nil {
		return LoginResult{}, fmt.Errorf("write audit notification: %w", err)
	}
	return result, nil
}

type LoginError struct {
	Message string
}

func (e *LoginError) Error() string { return e.Message }

func PaymentAction(amount float64, deviceFingerprint string) (string, string) {
	switch {
	case amount >= 10000:
		return "deny", "high_value_payment"
	case amount >= 1000 || deviceFingerprint == "":
		return "review", "additional_review_required"
	default:
		return "allow", "standard_payment"
	}
}
