package maxbot

import "testing"

func TestInitialAppUsername(t *testing.T) {
	tests := []struct {
		name     string
		username *string
		want     string
	}{
		{
			name: "MAX username",
			username: func() *string {
				value := "  andrey  "
				return &value
			}(),
			want: "andrey",
		},
		{
			name:     "missing username",
			username: nil,
			want:     "Не задано",
		},
		{
			name: "blank username",
			username: func() *string {
				value := " \t "
				return &value
			}(),
			want: "Не задано",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := initialAppUsername(test.username); got != test.want {
				t.Fatalf("initialAppUsername() = %q, want %q", got, test.want)
			}
		})
	}
}
