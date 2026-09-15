package services

import (
	"encoding/json"

	"github.com/mssola/user_agent"
)

type userAgent struct {
	Platform       string `json:"platform"`
	OSName         string `json:"osName"`
	OSVersion      string `json:"osVersion"`
	Mozilla        string `json:"mozilla"`
	Localization   string `json:"localization"`
	EngineName     string `json:"engineName"`
	EngineVersion  string `json:"engineVersion"`
	BrowserName    string `json:"browserName"`
	BrowserVersion string `json:"browserVersion"`
	Bot            bool   `json:"bot"`
	Mobile         bool   `json:"mobile"`
}

func userAgentFieldValue(userAgentHeader string) ([]byte, error) {
	ua := user_agent.New(userAgentHeader)
	osInf := ua.OSInfo()
	agent := userAgent{
		Platform:     ua.Platform(),
		OSName:       osInf.Name,
		OSVersion:    osInf.Version,
		Mozilla:      ua.Mozilla(),
		Mobile:       ua.Mobile(),
		Localization: ua.Localization(),
		Bot:          ua.Bot(),
	}
	agent.EngineName, agent.EngineVersion = ua.Engine()
	agent.BrowserName, agent.BrowserVersion = ua.Browser()

	return json.Marshal(&agent)
}
