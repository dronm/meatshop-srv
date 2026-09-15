package models

import "time"

type ProgAboutAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ProgAboutBackend struct {
	Version   string `json:"version"`
	BuildDate string `json:"build_date"`
	Commit    string `json:"commit"`
	IP        string `json:"ip"`
}

type ProgAboutDB struct {
	ServerType    string `json:"server_type"`
	ServerVersion string `json:"server_version"`
	Migration     string `json:"migration"`
}

type ProgAbout struct {
	Name      string           `json:"name"` // programm name
	Author    ProgAboutAuthor  `json:"author"`
	DB        *ProgAboutDB     `json:"db"`
	UpdatedAt time.Time        `json:"updated_at"`
	Backend   ProgAboutBackend `json:"backend"`
}
