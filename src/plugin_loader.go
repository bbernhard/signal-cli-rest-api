package main

import (
	"errors"
	"io"
	"net/http"
	"strings"

	gluasql "github.com/bbernhard/gluasql"
	"github.com/bbernhard/signal-cli-rest-api/api"
	"github.com/bbernhard/signal-cli-rest-api/utils"
	"github.com/cjoudrey/gluahttp"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	lua "github.com/yuin/gopher-lua"
	luajson "layeh.com/gopher-json"
	luar "layeh.com/gopher-luar"
)

type PluginInputData struct {
	Params      map[string]string
	QueryParams map[string]string
	Payload     string
}

type PluginOutputData struct {
	payload        string
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

func execPluginV1(c *gin.Context, pluginConfig utils.PluginConfig) {
	jsonData, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, api.Error{Msg: "Couldn't process request - invalid input data"})
		log.Error(err.Error())
		return
	}

	pluginInputData := &PluginInputData{
		Params:      make(map[string]string),
		QueryParams: make(map[string]string),
		Payload:     string(jsonData),
	}

	pluginOutputData := &PluginOutputData{
		payload:        "",
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
	gluasql.Preload(l)
	defer l.Close()
	if err := l.DoFile(pluginConfig.ScriptPath); err != nil {
		log.Error("Error executing lua script: ", err)
		c.JSON(400, api.Error{Msg: err.Error()})
		return
	}

	c.JSON(pluginOutputData.HttpStatusCode(), pluginOutputData.Payload())
}

func execPluginV2(c *gin.Context, pluginConfig utils.PluginConfig) {
	jsonData, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, api.Error{Msg: "Couldn't process request - invalid input data"})
		log.Error(err.Error())
		return
	}

	pluginInputData := &PluginInputData{
		Params:      make(map[string]string),
		QueryParams: make(map[string]string),
		Payload:     string(jsonData),
	}

	pluginOutputData := &PluginOutputData{
		payload:        "",
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
	gluasql.Preload(l)
	defer l.Close()
	if err := l.DoFile(pluginConfig.ScriptPath); err != nil {
		log.Error("Error executing lua script: ", err)
		c.JSON(400, api.Error{Msg: err.Error()})
		return
	}

	// Get global "exec"
	lv := l.GetGlobal("exec")

	// Check if it exists and is a function
	if fn, ok := lv.(*lua.LFunction); ok {
		err := l.CallByParam(lua.P{
			Fn:      fn,
			NRet:    1, // exec function returns one value
			Protect: true,
		})

		if err != nil {
			log.Error("Couldn't execute plugin: ", err.Error())
			c.JSON(400, "Couldn't execute plugin: "+err.Error())
			return
		}

		ret := l.Get(-1)
		l.Pop(1)

		if ret != lua.LNil {
			log.Error("Couldn't execute plugin")
			c.JSON(400, "Couldn't execute plugin")
		}
		c.Data(
			pluginOutputData.HttpStatusCode(),
			"application/json",
			[]byte(pluginOutputData.Payload()),
		)
	} else {
		log.Error("Couldn't execute plugin. No exec function implemented!")
		c.JSON(400, "Couldn't execute plugin. No exec function implemented!")
	}
}

type plugHandler struct {
}

func (p plugHandler) ExecutePlugin(pluginConfig utils.PluginConfig) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		if pluginConfig.Version == 1 {
			execPluginV1(c, pluginConfig)
		} else {
			execPluginV2(c, pluginConfig)
		}
	}

	return gin.HandlerFunc(fn)
}

func (p plugHandler) InitPlugin(pluginConfig utils.PluginConfig) error {
	l := lua.NewState()
	l.PreloadModule("http", gluahttp.NewHttpModule(&http.Client{}).Loader)
	luajson.Preload(l)
	gluasql.Preload(l)
	defer l.Close()
	err := l.DoFile(pluginConfig.ScriptPath)
	if err != nil {
		log.Error("Error executing lua script: ", err)
	}

	// Get global "init"
	lv := l.GetGlobal("init")

	// Check if it exists and is a function
	if fn, ok := lv.(*lua.LFunction); ok {
		err := l.CallByParam(lua.P{
			Fn:      fn,
			NRet:    2, // init function returns two values
			Protect: true,
		})

		if err != nil {
			return err
		}

		_ = l.Get(-2)
		errVal := l.Get(-1)
		l.Pop(2)

		if errVal != lua.LNil {
			return errors.New("Couldn't initialize lua script: " + errVal.String())
		}
	}

	return nil
}

// exported
var PluginHandler plugHandler