package box

import "testing"

// TestEventType_String tests the String method of EventType
func TestEventType_String(t *testing.T) {
	tests := []struct {
		name     string
		event    EventType
		expected string
	}{
		{
			name:     "Unknown event type 0",
			event:    EventType(0),
			expected: "Unknown Event",
		},
		{
			name:     "Unknown event type 99",
			event:    EventType(99),
			expected: "Unknown Event",
		},
		{
			name:     "Negative event type",
			event:    EventType(-1),
			expected: "Unknown Event",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.event.String()
			if result != tt.expected {
				t.Errorf("String() = %q, want %q", result, tt.expected)
			}
		})
	}
}
