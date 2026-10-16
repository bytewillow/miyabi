package api

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/ppxb/miyabi/internal/plex"
)

// PlexManager defines the contract for reading, saving, and testing Plex settings.
type PlexManager interface {
	Config(context.Context) (plex.Config, error)
	RetryPending(context.Context) error
	UpdateConfig(context.Context, plex.Config) error
	Test(context.Context, plex.Config) (plex.ServerInfo, error)
}

func plexConfigHandler(manager PlexManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := manager.Config(c.Request.Context())
		respond(c, cfg, err)
	}
}

func plexUpdateHandler(manager PlexManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, ok := bindJSON[plex.Config](c)
		if !ok {
			return
		}
		if err := manager.UpdateConfig(c.Request.Context(), cfg); err != nil {
			c.Error(err)
			return
		}
		updated, err := manager.Config(c.Request.Context())
		respond(c, updated, err)
	}
}

func plexTestHandler(manager PlexManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var cfg plex.Config
		if c.Request.ContentLength != 0 {
			bodyConfig, ok := bindJSON[plex.Config](c)
			if !ok {
				return
			}
			cfg = bodyConfig
		} else {
			var err error
			cfg, err = manager.Config(c.Request.Context())
			if err != nil {
				c.Error(err)
				return
			}
		}
		info, err := manager.Test(c.Request.Context(), cfg)
		respond(c, info, err)
	}
}
