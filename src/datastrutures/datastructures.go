package datastructures

type SendMessageRecipient struct {
	Identifier string `json:"identifier"`
	Type string `json:"type"`
}

type SendMessageV3 struct {
	Recipients        []SendMessageRecipient  `json:"recipients"`	
	Message           string                  `json:"message"`
	Base64Attachments []string                `json:"base64_attachments" example:"<BASE64 ENCODED DATA>,data:<MIME-TYPE>;base64<comma><BASE64 ENCODED DATA>,data:<MIME-TYPE>;filename=<FILENAME>;base64<comma><BASE64 ENCODED DATA>"`
	Sticker           string                  `json:"sticker"`
	Mentions          []client.MessageMention `json:"mentions"`
	QuoteTimestamp    *int64                  `json:"quote_timestamp"`
	QuoteAuthor       *string                 `json:"quote_author"`
	QuoteMessage      *string                 `json:"quote_message"`
	QuoteMentions     []client.MessageMention `json:"quote_mentions"`
	TextMode          *string                 `json:"text_mode" enums:"normal,styled"`
	EditTimestamp     *int64                  `json:"edit_timestamp"`
}
