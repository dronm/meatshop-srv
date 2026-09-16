package maxbot

import "testing"

func TestInitialAppUsername(t *testing.T) {
	tests := []struct {
		name      string
		username  *string
		firstName string
		want      string
	}{
		{
			name: "MAX username",
			username: func() *string {
				value := "  andrey  "
				return &value
			}(),
			firstName: "Andrey",
			want:      "andrey",
		},
		{
			name:      "first name fallback",
			firstName: "  Andrey  ",
			want:      "Andrey",
		},
		{
			name:      "blank username uses first name",
			firstName: "  Andrey  ",
			username: func() *string {
				value := " \t "
				return &value
			}(),
			want: "Andrey",
		},
		{
			name: "missing username and first name",
			want: "Не задано",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := initialAppUsername(test.username, test.firstName); got != test.want {
				t.Fatalf("initialAppUsername() = %q, want %q", got, test.want)
			}
		})
	}
}
