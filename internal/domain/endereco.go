package domain

import "time"

type Endereco struct {
	ID         string
	UserID     string
	Logradouro string
	Number     string
	Bairro     string
	Cep        string
	Latitude   float64
	Longitude  float64
	Cidade     string
	Estado     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
