package httpapi

import "testing"

func TestWireGuardHostAddress(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{input: "10.77.0.3/32", want: "10.77.0.3"},
		{input: "10.77.0.3", want: "10.77.0.3"},
	}
	for _, test := range tests {
		if got := wireGuardHostAddress(test.input); got != test.want {
			t.Fatalf("wireGuardHostAddress(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}
