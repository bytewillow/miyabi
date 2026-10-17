package api

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/ppxb/miyabi/internal/organize"
)

// OrganizeSettingsManager defines the contract for the organization mode setting.
type OrganizeSettingsManager interface {
	Config(context.Context) (organize.Config, error)
	UpdateConfig(context.Context, organize.Config) (organize.Config, error)
}

func organizeConfigHandler(manager OrganizeSettingsManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := manager.Config(c.Request.Context())
		respond(c, cfg, err)
	}
}

func organizeUpdateHandler(manager OrganizeSettingsManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, ok := bindJSON[organize.Config](c)
		if !ok {
			return
		}
		updated, err := manager.UpdateConfig(c.Request.Context(), cfg)
		respond(c, updated, err)
	}
}
