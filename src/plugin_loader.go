package main

import (
	"github.com/yuin/gopher-lua"
	"github.com/cjoudrey/gluahttp"
	"layeh.com/gopher-luar"
	luajson "layeh.com/gopher-json"
	"github.com/gin-gonic/gin"
	"io"
	log "github.com/sirupsen/logrus"
	"github.com/bbernhard/signal-cli-rest-api/utils"
	"github.com/bbernhard/signal-cli-rest-api/api"
	"strings"
	"net/http"
)

type PluginInputData struct {
	Params  map[string]string
	QueryParams map[string]string
	Payload string
}

type PluginOutputData struct {
	payload string
	httpStatusCode int
}

func (p *PluginOutputData) SetPayload(payload string) {
	p.payload = payload
}

func (p *PluginOutputData) Payload() string {
	return p.payload
}

func (p *PluginOutputData) SetHttpStatusCode(httpStatusCode int) {
	p.httpStatusCode = httpStatusCode
}

func (p *PluginOutputData) HttpStatusCode() int {
	return p.httpStatusCode
}

func execPlugin(c *gin.Context, pluginConfig utils.PluginConfig) {
	jsonData, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, api.Error{Msg: "Couldn't process request - invalid input data"})
		log.Error(err.Error())
		return
	}

	pluginInputData := &PluginInputData{
		Params: make(map[string]string),
		QueryParams: make(map[string]string),
		Payload: string(jsonData),
	}

	pluginOutputData := &PluginOutputData{
		payload: "",
		httpStatusCode: 200,
	}

	parts := strings.Split(pluginConfig.Endpoint, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ":") {
			paramName := strings.TrimPrefix(part, ":")
			pluginInputData.Params[paramName] = c.Param(paramName)
		}
	}

	queryParams := c.Request.URL.Query()
	for key, values := range queryParams {
		pluginInputData.QueryParams[key] = values[0]
	}

	l := lua.NewState()
	l.SetGlobal("pluginInputData", luar.New(l, pluginInputData))
	l.SetGlobal("pluginOutputData", luar.New(l, pluginOutputData))
	l.PreloadModule("http", gluahttp.NewHttpModule(&http.Client{}).Loader)
	luajson.Preload(l)
	defer l.Close()
	if err := l.DoFile(pluginConfig.ScriptPath); err != nil {
		c.JSON(400, api.Error{Msg: err.Error()})
		return
	}

	c.JSON(pluginOutputData.HttpStatusCode(), pluginOutputData.Payload())
}

type plugHandler struct {
}

func (p plugHandler) ExecutePlugin(pluginConfig utils.PluginConfig) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		execPlugin(c, pluginConfig)
	}

	return gin.HandlerFunc(fn)
}

//exported
var PluginHandler plugHandler
