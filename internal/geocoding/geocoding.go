// Package geocoding turns a postal address into coordinates, so clients can
// register an endereco without knowing its latitude/longitude.
package geocoding

import (
	"context"
	"errors"
)

var (
	// ErrNotFound means the provider has no match for the address.
	ErrNotFound = errors.New("geocoding: address not found")
	// ErrUnavailable means the provider could not be reached or failed.
	ErrUnavailable = errors.New("geocoding: provider unavailable")
)

type Address struct {
	Logradouro string
	Number     string
	Bairro     string
	Cidade     string
	Estado     string // UF, e.g. "SP"
	Cep        string
}

type Point struct {
	Latitude  float64
	Longitude float64
}

type Geocoder interface {
	Geocode(ctx context.Context, a Address) (Point, error)
}
