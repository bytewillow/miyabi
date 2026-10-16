package api

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/ppxb/miyabi/internal/strmurl"
)

// STRMSettingsManager defines the contract for the STRM base-URL setting.
type STRMSettingsManager interface {
	Config(context.Context) (strmurl.Response, error)
	UpdateConfig(context.Context, strmurl.Config) (strmurl.Response, error)
}

func strmConfigHandler(manager STRMSettingsManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := manager.Config(c.Request.Context())
		respond(c, cfg, err)
	}
}

func strmUpdateHandler(manager STRMSettingsManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, ok := bindJSON[strmurl.Config](c)
		if !ok {
			return
		}
		updated, err := manager.UpdateConfig(c.Request.Context(), cfg)
		respond(c, updated, err)
	}
}
