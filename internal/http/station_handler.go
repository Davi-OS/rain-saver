package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type stationResponse struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	UF        string  `json:"uf"`
}

func (s *Server) handleListStations(c *gin.Context) {
	stations, err := s.stations.List(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal error")
		return
	}
	out := make([]stationResponse, len(stations))
	for i, st := range stations {
		out[i] = stationResponse{Code: st.Code, Name: st.Name, Latitude: st.Latitude, Longitude: st.Longitude, UF: st.UF}
	}
	writeJSON(c, http.StatusOK, out)
}
