package data

import (
	"fmt"
)

type RecpType int

const (
	Number RecpType = iota + 1
	Username
	Group
)

type MessageMention struct {
	Start  int64  `json:"start"`
	Length int64  `json:"length"`
	Author string `json:"author"`
}

func (s *MessageMention) ToString() string {
	return fmt.Sprintf("%d:%d:%s", s.Start, s.Length, s.Author)
}

type SendMessageRecipient struct {
	Identifier string `json:"identifier"`
	Type       string `json:"type"`
}

type SignalCliSendRequest struct {
	Number            string
	Message           string
	Recipients        []string
	Base64Attachments []string
	RecipientType     RecpType
	Sticker           string
	Mentions          []MessageMention
	QuoteTimestamp    *int64
	QuoteAuthor       *string
	QuoteMessage      *string
	QuoteMentions     []MessageMention
	TextMode          *string
	EditTimestamp     *int64
}
