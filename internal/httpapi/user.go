package httpapi

import (
	"fmt"
	"net/http"

	"github.com/dronm/modelbind"
	"github.com/dronm/meatshop/internal/models"
	"github.com/dronm/webapp"
)

func userRoutes(api *webapp.Group) {
	api.POST(
		"/users",
		webapp.WithName("user.create"),
		webapp.WithPermission("user.create"),
		webapp.WithService("User", "Create"),
		webapp.WithBinder(webapp.InsertInputBinder[*models.User]()),
		webapp.WithSuccessCode(http.StatusCreated),
	)

	api.GET(
		"/users",
		webapp.WithName("user.list"),
		webapp.WithPermission("user.list"),
		webapp.WithService("User", "List"),
		webapp.WithBinder(webapp.CollectionParamsBinder()),
	)

	api.GET(
		"/users/me",
		webapp.WithName("user.profile.detail"),
		webapp.WithPermission("user.profile.detail"),
		webapp.WithService("User", "CurrentProfile"),
	)

	api.PATCH(
		"/users/me",
		webapp.WithName("user.profile.update"),
		webapp.WithPermission("user.profile.update"),
		webapp.WithService("User", "UpdateCurrentProfile"),
		webapp.WithBinder(UserProfileUpdateBinder()),
	)

	api.PATCH(
		"/users/me/password",
		webapp.WithName("user.profile.password"),
		webapp.WithPermission("user.profile.password"),
		webapp.WithService("User", "ChangeCurrentPassword"),
		webapp.WithBinder(UserPasswordChangeBinder()),
	)

	api.GET(
		"/users/{id}",
		webapp.WithName("user.detail"),
		webapp.WithPermission("user.detail"),
		webapp.WithService("User", "Detail"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
	)

	api.PATCH(
		"/users/{id}",
		webapp.WithName("user.update"),
		webapp.WithPermission("user.update"),
		webapp.WithService("User", "Update"),
		webapp.WithBinder(webapp.UpdateByPathKeysBinder[*models.UserKey, *models.UserUpdate]("id")),
	)

	api.DELETE(
		"/users/{id}",
		webapp.WithName("user.delete"),
		webapp.WithPermission("user.delete"),
		webapp.WithService("User", "Delete"),
		webapp.WithBinder(webapp.PathValueBinder[int]("id")),
	)

	//public method
	api.POST(
		"/users/login",
		webapp.WithName("user.login"),
		webapp.WithService("User", "Login"),
		webapp.WithBinder(UserLoginBinder()),
	)

	api.POST(
		"/users/logout",
		webapp.WithName("user.logout"),
		webapp.WithPermission("user.logout"),
		webapp.WithService("User", "Logout"),
	)
}

func UserLoginBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		bodyInput, err := modelbind.DecodeRequestInput[*models.UserLoginRequest](r)
		if err != nil {
			return nil, fmt.Errorf("decode login request: %w", err)
		}

		if err := bodyInput.Validate(false); err != nil {
			return nil, fmt.Errorf("validate login request: %w", err)
		}

		if bodyInput.Model == nil {
			return nil, fmt.Errorf("login request is empty")
		}

		return models.UserLoginInput{
			UserInf: models.UserLoginInf{
				Headers:    r.Header.Clone(),
				RemoteAddr: r.RemoteAddr,
			},
			Model: *bodyInput.Model,
		}, nil
	}
}

func UserProfileUpdateBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		bodyInput, err := modelbind.DecodeRequestInput[*models.UserProfileUpdate](r)
		if err != nil {
			return nil, fmt.Errorf("decode user profile request: %w", err)
		}

		if err := bodyInput.Validate(false); err != nil {
			return nil, fmt.Errorf("validate user profile request: %w", err)
		}

		if bodyInput.Model == nil {
			return nil, fmt.Errorf("user profile request is empty")
		}

		return *bodyInput.Model, nil
	}
}

func UserPasswordChangeBinder() webapp.Binder {
	return func(r *http.Request) (any, error) {
		bodyInput, err := modelbind.DecodeRequestInput[*models.UserPasswordChange](r)
		if err != nil {
			return nil, fmt.Errorf("decode password change request: %w", err)
		}

		if err := bodyInput.Validate(false); err != nil {
			return nil, fmt.Errorf("validate password change request: %w", err)
		}

		if bodyInput.Model == nil {
			return nil, fmt.Errorf("password change request is empty")
		}

		return *bodyInput.Model, nil
	}
}
