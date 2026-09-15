package models

// Ref is a common ref field for mose models
// where there is one key of type int.
type Ref struct {
	Keys struct {
		ID int `json:"id"`
	} `json:"keys"`
	Descr string `json:"descr"`
}
