package api

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"fmt"

	_ "runtime/debug"
	_ "github.com/yassinebenaid/godump"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"github.com/bbernhard/signal-cli-rest-api/client"
)

type AlertManagerNotification struct {
	Receiver          string      `json:"receiver"`
	Status            string      `json:"status"`
	Alerts            []Alert     `json:"alerts"`
	GroupLabels       Labels      `json:"groupLabels"`
	CommonLabels      Labels      `json:"commonLabels"`
	CommonAnnotations Annotations `json:"commonAnnotations"`
	ExternalURL       string      `json:"externalURL"`
	Version           string      `json:"version"`
	GroupKey          string      `json:"groupKey"`
	TruncatedAlerts   int64       `json:"truncatedAlerts"`
	OrgID             int64       `json:"orgId"`
	Title             string      `json:"title"`
	State             string      `json:"state"`
	Message           string      `json:"message"`
}

type Alert struct {
	Status       string      `json:"status"`
	Labels       Labels      `json:"labels"`
	Annotations  Annotations `json:"annotations"`
	StartsAt     string      `json:"startsAt"`
	EndsAt       string      `json:"endsAt"`
	GeneratorURL string      `json:"generatorURL"`
	Fingerprint  string      `json:"fingerprint"`
	SilenceURL   string      `json:"silenceURL"`
	DashboardURL string      `json:"dashboardURL"`
	PanelURL     string      `json:"panelURL"`
	Values       interface{} `json:"values"`
	ValueString  string      `json:"valueString"`
}

type Annotations struct {
	Summary string `json:"summary"`
}

type Labels struct {
	Alertname string `json:"alertname"`
	Instance  string `json:"instance"`
}

type GrafanaMessage struct {
	Number           string   `json:"number"`
	Recipients       string   `json:"recipients"`
	Message          string   `json:"message"`
}

type GraylogNotification struct {
	EventDefinitionID          string        `json:"event_definition_id"`
	EventDefinitionType        string        `json:"event_definition_type"`
	EventDefinitionTitle       string        `json:"event_definition_title"`
	EventDefinitionDescription string        `json:"event_definition_description"`
	JobDefinitionID            string        `json:"job_definition_id"`
	JobTriggerID               string        `json:"job_trigger_id"`
	Event                      GraylogEvent  `json:"event"`
	Backlog                    []interface{} `json:"backlog"`
}

type GraylogEvent struct {
	ID                  string        `json:"id"`
	EventDefinitionType string        `json:"event_definition_type"`
	EventDefinitionID   string        `json:"event_definition_id"`
	OriginContext       string        `json:"origin_context"`
	Timestamp           string        `json:"timestamp"`
	TimestampProcessing string        `json:"timestamp_processing"`
	TimerangeStart      interface{}   `json:"timerange_start"`
	TimerangeEnd        interface{}   `json:"timerange_end"`
	Streams             []string      `json:"streams"`
	SourceStreams       []interface{} `json:"source_streams"`
	Message             string        `json:"message"`
	Source              string        `json:"source"`
	KeyTuple            []string      `json:"key_tuple"`
	Key                 string        `json:"key"`
	Priority            int64         `json:"priority"`
	Alert               bool          `json:"alert"`
	Fields              Fields        `json:"fields"`
}

type Fields struct {
	Recipients string `json:"recipients"`
	FromNumber string `json:"fromnumber"`
	Message    string `json:"message"`
}

// @Summary Send a signal message.
// @Tags Messages
// @Description Send a signal message. Set the text_mode to 'styled' in case you want to add formatting to your text message. Styling Options: *italic text*, **bold text**, ~strikethrough text~.
// @Accept  json
// @Produce  json
// @Success 201 {object} SendMessageResponse
// @Failure 400 {object} SendMessageError
// @Param data body SendMessageV2 true "Input Data"
// @Router /v2/send [post]
func (a *Api) SendAlertManagerV2(c *gin.Context) {
	var req AlertManagerNotification
	var msg GrafanaMessage
	base64Attachments := []string{}

	err := c.BindJSON(&req)
	if err != nil {
		c.JSON(400, gin.H{"error": "Couldn't process request - invalid request"})
		log.Error(err.Error())
		return
	}

	//fmt.Printf(">>>%s\n",[]byte(req.Message))

	// Unmarshal or Decode the JSON to the interface.
	json.Unmarshal([]byte(req.Message), &msg)

//	timestamp, err := a.signalClient.SendV1(msg.Number, msg.Message, msg.Recipients, base64Attachments, msg.IsGroup)
//	if err != nil {
//		c.JSON(400, Error{Msg: err.Error()})
//		return
//	}
//	c.JSON(201, SendMessageResponse{Timestamp: strconv.FormatInt(timestamp.Timestamp, 10)})

        data, err := a.signalClient.SendV2(msg.Number, msg.Message, strings.Split(msg.Recipients,","), base64Attachments, "", nil, nil, nil, nil, nil, nil, nil, nil)

        if err != nil {
                switch err.(type) {
                case *client.RateLimitErrorType:
                        if rateLimitError, ok := err.(*client.RateLimitErrorType); ok {
                                extendedError := errors.New(err.Error() + ". Use the attached challenge tokens to lift the rate limit restrictions via the '/v1/accounts/{number}/rate-limit-challenge' endpoint.")
                                c.JSON(429, SendMessageError{Msg: extendedError.Error(), ChallengeTokens: rateLimitError.ChallengeTokens, Account: msg.Number})
                                return
                        } else {
                                c.JSON(400, Error{Msg: err.Error()})
                                return
                        }
                default:
                        c.JSON(400, Error{Msg: err.Error()})
                        return
                }
                c.JSON(400, Error{Msg: err.Error()})
                return
        }

        c.JSON(201, SendMessageResponse{Timestamp: strconv.FormatInt((*data)[0].Timestamp, 10)})

}

// @Summary Send a signal message.
// @Tags Messages
// @Description Send a signal message. Set the text_mode to 'styled' in case you want to add formatting to your text message. Styling Options: *italic text*, **bold text**, ~strikethrough text~.
// @Accept  json
// @Produce  json
// @Success 201 {object} SendMessageResponse
// @Failure 400 {object} SendMessageError
// @Param data body SendMessageV2 true "Input Data"
// @Router /v2/send [post]
func (a *Api) SendGraylogNotificationV2(c *gin.Context) {
        var req GraylogNotification
        base64Attachments := []string{}
       // jsonData,err2 := io.ReadAll(c.Request.Body)
	//if err2 != nil {
	//	log.Error(err2.Error())
	//}
	//fmt.Printf("<<<%s\n",jsonData)

        err := c.BindJSON(&req)
        if err != nil {
                c.JSON(400, gin.H{"error": "Couldn't process request - invalid requestttttt"})
                log.Error(err.Error())
		fmt.Printf("<<<%s\n",c.Request.Body)
                return
        }

	data, err := a.signalClient.SendV2(req.Event.Fields.FromNumber, req.Event.Fields.Message, strings.Split(req.Event.Fields.Recipients,","), base64Attachments, "", nil, nil, nil, nil, nil, nil, nil, nil)

        if err != nil {
                switch err.(type) {
                case *client.RateLimitErrorType:
                        if rateLimitError, ok := err.(*client.RateLimitErrorType); ok {
                                extendedError := errors.New(err.Error() + ". Use the attached challenge tokens to lift the rate limit restrictions via the '/v1/accounts/{number}/rate-limit-challenge' endpoint.")
                                c.JSON(429, SendMessageError{Msg: extendedError.Error(), ChallengeTokens: rateLimitError.ChallengeTokens, Account: req.Event.Fields.FromNumber})
                                return
                        } else {
                                c.JSON(400, Error{Msg: err.Error()})
                                return
                        }
                default:
                        c.JSON(400, Error{Msg: err.Error()})
                        return
                }
                c.JSON(400, Error{Msg: err.Error()})
                return
        }

        c.JSON(201, SendMessageResponse{Timestamp: strconv.FormatInt((*data)[0].Timestamp, 10)})
}


