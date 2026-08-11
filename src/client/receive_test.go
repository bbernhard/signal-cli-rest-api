package client

import (
	"encoding/json"
	"testing"

	ds "github.com/bbernhard/signal-cli-rest-api/datastructs"
	utils "github.com/bbernhard/signal-cli-rest-api/utils"
)

func TestMarshalJsonStream(t *testing.T) {
	output, err := marshalJsonStream("{\"account\":\"one\"}\n{\"account\":\"two\"}\n")
	if err != nil {
		t.Fatal(err)
	}

	var messages []map[string]string
	if err := json.Unmarshal([]byte(output), &messages); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[0]["account"] != "one" || messages[1]["account"] != "two" {
		t.Fatalf("unexpected messages: %#v", messages)
	}
}

func TestMarshalJsonStreamRejectsTrailingStderr(t *testing.T) {
	_, err := marshalJsonStream("{\"account\":\"one\"}\njava.lang.Throwable\n")
	if err == nil {
		t.Fatal("expected malformed trailing output to be rejected")
	}
}

func TestValidateBodyRangesUsesFinalUTF16Length(t *testing.T) {
	parser := utils.NewTextstyleParser("👋 **hello**")
	message, styles := parser.Parse()
	mentions := []ds.MessageMention{{Start: 3, Length: 5, Author: "aci"}}

	if err := validateBodyRanges(message, mentions, styles); err != nil {
		t.Fatal(err)
	}
}

func TestValidateBodyRangesRejectsUTF8ByteOffsets(t *testing.T) {
	message := "👋 hello"
	mentions := []ds.MessageMention{{Start: 5, Length: 5, Author: "aci"}}

	if err := validateBodyRanges(message, mentions, nil); err == nil {
		t.Fatal("expected UTF-8 byte offset to exceed the UTF-16 message length")
	}
}
