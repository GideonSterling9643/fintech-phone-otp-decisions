package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	otpfintech "github.com/example/fintech-phone-otp"
)

func main() {
	client := otpfintech.NewClient(os.Getenv("INFRAI_API_KEY"))
	workflow := otpfintech.PaymentLogin{
		Gateway: client,
		OTP:     otpfintech.StaticOTPVerifier{Code: envOr("OTP_CODE", "123456")},
		Audit:   os.Stdout,
		Now:     time.Now,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /payment-login/verify", func(w http.ResponseWriter, r *http.Request) {
		var in otpfintech.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.RequestID == "" || in.Phone == "" || in.Code == "" || in.CaptchaToken == "" || in.WidgetRecordID == "" || in.AccountID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request_id, phone, code, captcha_token, and account_id are required"})
			return
		}
		result, err := workflow.Verify(r.Context(), in)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
	addr := envOr("ADDR", ":8080")
	log.Printf("payment OTP service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeAPIError(w http.ResponseWriter, err error) {
	var loginErr *otpfintech.LoginError
	if errors.As(err, &loginErr) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_code", "message": loginErr.Message})
		return
	}
	var apiErr *otpfintech.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		writeJSON(w, apiErr.HTTPStatus, map[string]string{"error": apiErr.Code, "message": apiErr.Message})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream_request_failed"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
