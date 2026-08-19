package service

import "errors"

var (
	// ErrEnderecoNotFound covers both "no such address" and "address
	// belongs to a different user" — the same sentinel either way, so
	// handlers can't leak which case occurred (avoids ID enumeration).
	ErrEnderecoNotFound = errors.New("service: endereco not found")
	ErrWeatherProvider  = errors.New("service: weather provider error")
)
