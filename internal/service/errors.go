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
	// ErrAddressNotFound means the address has no coordinates and the
	// geocoder could not locate it.
	ErrAddressNotFound = errors.New("service: address could not be located")
	// ErrGeocodingUnavailable means the geocoding provider failed or is
	// unreachable; the client may retry later or send coordinates itself.
	ErrGeocodingUnavailable = errors.New("service: geocoding unavailable")
)
