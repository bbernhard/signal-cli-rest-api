package utils

import (
	"bytes"
	"encoding/json"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// relative to the envelope
var styledTextFields = []struct{ object, textKey, styledKey string }{
	{"dataMessage", "message", "styledMessage"},
	{"dataMessage.quote", "text", "styledText"},
	{"editMessage.dataMessage", "message", "styledMessage"},
	{"editMessage.dataMessage.quote", "text", "styledText"},
	{"syncMessage.sentMessage", "message", "styledMessage"},
	{"syncMessage.sentMessage.quote", "text", "styledText"},
	{"syncMessage.sentMessage.editMessage.dataMessage", "message", "styledMessage"},
	{"syncMessage.sentMessage.editMessage.dataMessage.quote", "text", "styledText"},
}

// AddStyledMessages adds the text in the markup of text_mode 'styled' to a received message,
// as styledMessage next to message and styledText next to the text of quotes.
func AddStyledMessages(payload []byte) []byte {
	for _, field := range styledTextFields {
		object := "envelope." + field.object + "."
		text := gjson.GetBytes(payload, object+field.textKey)
		if text.Type != gjson.String || text.Str == "" {
			continue
		}
		var styles []TextStyleRange
		if rawStyles := gjson.GetBytes(payload, object+"textStyles"); rawStyles.IsArray() {
			_ = json.Unmarshal([]byte(rawStyles.Raw), &styles)
		}
		if updated, err := sjson.SetRawBytes(payload, object+field.styledKey, jsonString(FormatTextStyles(text.Str, styles))); err == nil {
			payload = updated
		}
	}
	return payload
}

// sjson.SetBytes would escape HTML characters
func jsonString(s string) []byte {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(s)
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}
