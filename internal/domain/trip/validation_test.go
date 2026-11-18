package trip

import "testing"

func TestValidationError(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		reason   string
		expected string
	}{
		{
			name:     "invalid price",
			field:    "price",
			reason:   "must be nonnegative",
			expected: "price: must be nonnegative",
		},
		{
			name:     "invalid coordinates",
			field:    "start_point",
			reason:   "invalid coordinates",
			expected: "start_point: invalid coordinates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ValidationError{
				Field:  tt.field,
				Reason: tt.reason,
			}

			if got := err.Error(); got != tt.expected {
				t.Errorf("Ожидали %q, получили %q", tt.expected, got)
			}
		})
	}
}
