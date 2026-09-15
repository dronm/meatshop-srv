package models

// MainMenuForUser is the hierarchical menu returned to the authenticated client.
type MainMenuForUser struct {
	ID       int                `json:"id"`
	Caption  string             `json:"caption"`
	Route    string             `json:"route"`
	Icon     *string            `json:"icon"`
	Children []*MainMenuForUser `json:"children"`
}
