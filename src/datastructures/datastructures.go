package datastructures

const GroupPrefix = "group."

type GroupEntry struct {
	Name       string   `json:"name"`
	Id         string   `json:"id"`
	InternalId string   `json:"internal_id"`
	Members    []string `json:"members"`
	Active     bool     `json:"active"`
	Blocked    bool     `json:"blocked"`
}

type RegisterNumberRequest struct {
	UseVoice bool `json:"use_voice"`
}

type VerifyNumberSettings struct {
	Pin string `json:"pin"`
}

type SendMessageV1 struct {
	Number           string   `json:"number"`
	Recipients       []string `json:"recipients"`
	Message          string   `json:"message"`
	Base64Attachment string   `json:"base64_attachment"`
	IsGroup          bool     `json:"is_group"`
}

type SendMessageV2 struct {
	Number            string   `json:"number"`
	Recipients        []string `json:"recipients"`
	Message           string   `json:"message"`
	Base64Attachments []string `json:"base64_attachments"`
}

type Error struct {
	Msg string `json:"error"`
}

type About struct {
	SupportedApiVersions []string `json:"versions"`
	BuildNr              int      `json:"build"`
}

type CreateGroup struct {
	Id string `json:"id"`
}

