package main

import (
	"fmt"
	"net/http"
)

type APIError struct {
	StatusCode int `json:"-"`
	Msg        any `json:"message"`
}

func (e APIError) Error() string {
	return fmt.Sprintf("api error: %d", e.StatusCode)
}

func NewAPIError(status int, msg any) APIError {
	return APIError{
		StatusCode: status,
		Msg: msg,
	}
}

func InternalError() APIError {
	return APIError{
		StatusCode: http.StatusInternalServerError,
		Msg: "internal error",
	}
}

func BadRequest() APIError {
	return APIError{
		StatusCode: http.StatusBadRequest,
		Msg: "bad request",
	}
}

func InvalidJSONRequestData(errors map[string]string) APIError {
	return APIError{
		StatusCode: http.StatusUnprocessableEntity,
		Msg: errors,
	}
}

func UserNotAuthenticated() APIError {
	return APIError{
		StatusCode: http.StatusUnauthorized,
		Msg: "user not authenticated",
	}
}

func InvalidToken() APIError {
	return APIError{
		StatusCode: http.StatusUnauthorized,
		Msg: "Invalid Token",
	}
}

func AccessNotAllowed() APIError {
	return APIError{
		StatusCode: http.StatusUnauthorized,
		Msg: "You do not have the necessary permissions to access the requested content",
	}
}

func NotImplemented() APIError {
	return APIError{
		StatusCode: http.StatusNotImplemented,
		Msg: "Endpoint not implemented",
	}
}
