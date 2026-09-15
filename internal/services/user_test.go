package services

import "testing"

func TestHashUserPassword(t *testing.T) {
	t.Parallel()

	got := hashUserPassword("123456")
	const want = "e10adc3949ba59abbe56e057f20f883e"
	if got != want {
		t.Fatalf("hashUserPassword() = %q, want %q", got, want)
	}
}
