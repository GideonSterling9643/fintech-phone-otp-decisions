package otpfintech

import "testing"

func TestPaymentAction(t *testing.T) {
	tests := []struct {
		name        string
		amount      float64
		fingerprint string
		wantAction  string
		wantReason  string
	}{
		{"known device small payment", 85, "device-7", "allow", "standard_payment"},
		{"unknown device", 85, "", "review", "additional_review_required"},
		{"large payment", 2500, "device-7", "review", "additional_review_required"},
		{"very large payment", 12000, "device-7", "deny", "high_value_payment"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, reason := PaymentAction(tt.amount, tt.fingerprint)
			if action != tt.wantAction || reason != tt.wantReason {
				t.Fatalf("PaymentAction(%v, %q) = %q, %q; want %q, %q", tt.amount, tt.fingerprint, action, reason, tt.wantAction, tt.wantReason)
			}
		})
	}
}
