package models

import "github.com/dronm/modelbind/metadata"

const (
	ModelbindEnumRoleID = "role_id"
)

// RegisterModelbindEnums registers every application enum before model metadata
// is parsed and used concurrently.
func RegisterModelbindEnums() {
	if metadata.Enums == nil {
		metadata.Enums = make(map[string][]string, 10)
	}

	metadata.Enums[ModelbindEnumRoleID] = RoleIDValues()
}
