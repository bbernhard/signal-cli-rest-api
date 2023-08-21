package utils

import (
	"strconv"
)

const (
	Normal string = "NORMAL"
	Bold          = "BOLD"
	Italic        = "ITALIC"
)

const (
	None        int = 0
	BoldBegin       = 1
	BoldEnd         = 2
	ItalicBegin     = 3
	ItalicEnd1      = 4
	ItalicEnd2      = 5
)

func ParseMarkdownMessage(message string) (string, []string) {
	textFormat := Normal
	textFormatBegin := 0
	textFormatEnd := 0
	state := None
	signalCliFormatStrings := []string{}
	fullString := ""

	runes := []rune(message) //turn string to slice

	for i, v := range runes { //iterate through rune
		if v == '*' {
			if state == BoldBegin {
				if i-1 == textFormatBegin {
					state = ItalicBegin
					textFormat = Italic
					textFormatBegin = i
				} else {
					state = BoldEnd
					textFormatEnd = i - 1
				}
			} else if state == None {
				state = BoldBegin
				textFormat = Bold
				textFormatBegin = i
			} else if state == ItalicBegin {
				state = ItalicEnd1
				textFormatEnd = i - 1
			} else if state == ItalicEnd1 {
				state = ItalicEnd2
			}
		} else {
			fullString += string(v)
		}

		if state == BoldEnd || state == ItalicEnd2 {	
			signalCliFormatStrings = append(signalCliFormatStrings, strconv.Itoa(textFormatBegin)+":"+strconv.Itoa(textFormatEnd-textFormatBegin)+":"+textFormat)
			state = None
			textFormatBegin = 0
			textFormatEnd = 0
			textFormat = Normal
		}
	}

	return fullString, signalCliFormatStrings
}
