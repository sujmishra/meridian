package server

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed ui/index.html
var dashboardHTML []byte

func (s *Server) handleDashboard(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", dashboardHTML)
}
