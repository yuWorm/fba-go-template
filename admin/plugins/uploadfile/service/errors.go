package service

import (
	"net/http"

	fbaerrors "github.com/yuWorm/fba-go/core/errors"
)

func badRequest(message string, cause error) error {
	return fbaerrors.New(http.StatusBadRequest, http.StatusBadRequest, message, cause)
}

func notFound(message string, cause error) error {
	return fbaerrors.New(http.StatusNotFound, http.StatusNotFound, message, cause)
}

func forbidden(message string) error {
	return fbaerrors.New(http.StatusForbidden, http.StatusForbidden, message, nil)
}
