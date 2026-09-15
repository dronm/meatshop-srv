package models

type MainMenuRouteAutocompleteInput struct {
	Query string
	Limit int
}

type MainMenuRouteOption struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	Path    string  `json:"path"`
	Descr   string  `json:"descr"`
	Section string  `json:"section"`
	Icon    *string `json:"icon,omitempty"`
}
