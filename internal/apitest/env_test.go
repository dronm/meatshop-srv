//go:build integration

package apitest

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if err := loadProjectDotEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "load integration test environment: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}
