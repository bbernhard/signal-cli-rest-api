package utils

import (
	"github.com/gin-gonic/gin"
)

type PluginHandler interface {
	ExecutePlugin(pluginConfig PluginConfig) gin.HandlerFunc
}
