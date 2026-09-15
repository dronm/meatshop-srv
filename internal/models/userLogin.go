package models

import (
	"net/http"
	"time"

	wmodels "github.com/dronm/webapp/models"
)

type UserLogin struct {
	ID       int       `json:"id"`
	Name     string    `json:"name"`
	RoleID   RoleID    `json:"role_id"`
	CreateDt time.Time `json:"create_dt"`
	Pwd      string    `f:"pwd" json:"-"`
	Banned   bool      `f:"banned" json:"-"`
	PubKey   string    `f:"-" json:"pub_key"`
	LoginID  int       `f:"-" json:"-"`
	BanHash  *string   `f:"ban_hash" json:"-"`
}

type UserLoginRequest struct {
	Name string `json:"name" required:"true" maxLen:"100"`
	Pwd  string `json:"pwd" required:"true" maxLen:"50"`
}

type UserLoginInput struct {
	UserInf UserLoginInf
	Model   UserLoginRequest
}

type UserLoginResponse struct {
	User *UserLogin    `json:"user"`
	Auth *wmodels.Auth `json:"auth"`
}

type UserLoginInf struct {
	Headers    http.Header
	RemoteAddr string
}
