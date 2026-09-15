package models

type RoleID string

const (
	RoleIDAdmin RoleID = "admin"
)

func RoleIDValues() []string {
	return []string{
		string(RoleIDAdmin),
	}
}

func (v RoleID) IsValid() bool {
	switch v {
	case RoleIDAdmin:
		return true
	default:
		return false
	}
}

func (v RoleID) String() string {
	return string(v)
}
