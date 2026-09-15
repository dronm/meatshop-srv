package models

import (
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/dronm/modelbind"
	"github.com/dronm/modelbind/metadata"
)

func TestRegisterModelbindEnums(t *testing.T) {
	originalEnums := metadata.Enums
	metadata.Enums = nil
	t.Cleanup(func() {
		metadata.Enums = originalEnums
	})

	RegisterModelbindEnums()

	tests := map[string][]string{
		ModelbindEnumRoleID:                             RoleIDValues(),
	}

	for enumID, expected := range tests {
		actual, ok := metadata.Enums[enumID]
		if !ok {
			t.Fatalf("enum %q is not registered", enumID)
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("enum %q values: got %v, want %v", enumID, actual, expected)
		}
	}
}

func TestModelbindValidatesNamedStringEnums(t *testing.T) {
	RegisterModelbindEnums()

	t.Run("role", func(t *testing.T) {
		validInput, err := decodeUserInput(`{"role_id":"admin"}`)
		if err != nil {
			t.Fatalf("decode valid role: %v", err)
		}
		if err := validInput.Validate(false); err != nil {
			t.Fatalf("validate valid role: %v", err)
		}

		invalidInput, err := decodeUserInput(`{"role_id":"invalid"}`)
		if err != nil {
			t.Fatalf("decode invalid role: %v", err)
		}
		if err := invalidInput.Validate(false); err == nil {
			t.Fatal("invalid role must fail validation")
		}
	})

	t.Run("nullable role", func(t *testing.T) {
		validInput, err := decodeMainMenuInput(`{"role_id":"admin"}`)
		if err != nil {
			t.Fatalf("decode valid nullable role: %v", err)
		}
		if err := validInput.Validate(false); err != nil {
			t.Fatalf("validate valid nullable role: %v", err)
		}

		invalidInput, err := decodeMainMenuInput(`{"role_id":"invalid"}`)
		if err != nil {
			t.Fatalf("decode invalid nullable role: %v", err)
		}
		if err := invalidInput.Validate(false); err == nil {
			t.Fatal("invalid nullable role must fail validation")
		}
	})

}

func TestEnumValuesAreValid(t *testing.T) {
	for _, value := range []RoleID{RoleIDAdmin} {
		if !value.IsValid() {
			t.Fatalf("role %q must be valid", value)
		}
	}

}

func decodeUserInput(body string) (modelbind.ModelInput[*User], error) {
	request := httptest.NewRequest("PATCH", "/users/1", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	return modelbind.DecodeRequestInput[*User](request)
}

func decodeMainMenuInput(body string) (modelbind.ModelInput[*MainMenu], error) {
	request := httptest.NewRequest("PATCH", "/main-menus/1", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	return modelbind.DecodeRequestInput[*MainMenu](request)
}

