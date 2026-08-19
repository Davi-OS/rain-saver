package service

import "errors"

var (
	// ErrEnderecoNotFound covers both "no such address" and "address
	// belongs to a different user" — the same sentinel either way, so
	// handlers can't leak which case occurred (avoids ID enumeration).
	ErrEnderecoNotFound = errors.New("service: endereco not found")
	// ErrNoStationAvailable means no weather station is registered at all
	// — the ingestion side hasn't populated weather_stations yet.
	ErrNoStationAvailable = errors.New("service: no weather station available")
)
