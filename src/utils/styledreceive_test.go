package utils

import (
	"encoding/json"
	"reflect"
	"testing"
)

func decodeJson(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid json %q: %v", data, err)
	}
	return result
}

func TestAddStyledMessagesDataMessage(t *testing.T) {
	payload := `{"envelope":{"source":"+431212131491291","timestamp":1700000000000,"dataMessage":{"timestamp":1700000000000,` +
		`"message":"bold and 5 * 3 <x>","textStyles":[{"style":"BOLD","start":0,"length":4}],` +
		`"quote":{"id":1699999999999,"text":"quoted italic","textStyles":[{"style":"ITALIC","start":7,"length":6}]}}},` +
		`"account":"+4354546464654"}`

	got := decodeJson(t, AddStyledMessages([]byte(payload)))
	want := decodeJson(t, []byte(payload))
	dataMessage := want["envelope"].(map[string]any)["dataMessage"].(map[string]any)
	dataMessage["styledMessage"] = `**bold** and 5 \* 3 <x>`
	dataMessage["quote"].(map[string]any)["styledText"] = "quoted *italic*"

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwanted %v", got, want)
	}
}

func TestAddStyledMessagesWithoutStyles(t *testing.T) {
	payload := `{"envelope":{"dataMessage":{"timestamp":1,"message":"*not italic*"}},"account":"+1"}`
	got := decodeJson(t, AddStyledMessages([]byte(payload)))
	styled := got["envelope"].(map[string]any)["dataMessage"].(map[string]any)["styledMessage"]
	if styled != `\*not italic\*` {
		t.Errorf("got %q", styled)
	}
}

func TestAddStyledMessagesSyncAndEdit(t *testing.T) {
	payload := `{"envelope":{` +
		`"editMessage":{"targetSentTimestamp":1,"dataMessage":{"timestamp":2,"message":"edited","textStyles":[{"style":"STRIKETHROUGH","start":0,"length":6}]}},` +
		`"syncMessage":{"sentMessage":{"timestamp":3,"message":"sent","textStyles":[{"style":"SPOILER","start":0,"length":4}],` +
		`"editMessage":{"targetSentTimestamp":3,"dataMessage":{"timestamp":4,"message":"sent edit","textStyles":[{"style":"MONOSPACE","start":5,"length":4}]}}}}},` +
		`"account":"+1"}`

	envelope := decodeJson(t, AddStyledMessages([]byte(payload)))["envelope"].(map[string]any)
	sentMessage := envelope["syncMessage"].(map[string]any)["sentMessage"].(map[string]any)
	for _, tc := range []struct {
		object map[string]any
		want   string
	}{
		{envelope["editMessage"].(map[string]any)["dataMessage"].(map[string]any), "~edited~"},
		{sentMessage, "||sent||"},
		{sentMessage["editMessage"].(map[string]any)["dataMessage"].(map[string]any), "sent `edit`"},
	} {
		if tc.object["styledMessage"] != tc.want {
			t.Errorf("got %q, wanted %q", tc.object["styledMessage"], tc.want)
		}
	}
}

func TestAddStyledMessagesKeepsOtherPayloads(t *testing.T) {
	for _, payload := range []string{
		`{"envelope":{"receiptMessage":{"when":1,"isDelivery":true}},"account":"+1"}`,
		`{"envelope":{"dataMessage":{"timestamp":1,"message":null}},"account":"+1"}`,
		`{"jsonrpc":"2.0","result":{"timestamp":1},"id":"x"}`,
		`not json`,
		``,
	} {
		if got := string(AddStyledMessages([]byte(payload))); got != payload {
			t.Errorf("got %q, wanted it unchanged", got)
		}
	}
}

func TestAddStyledMessagesKeepsTheRestOfThePayload(t *testing.T) {
	payload := `{"envelope":{"timestamp":1700000000000123,"dataMessage":{"timestamp":1700000000000123,"message":"hi 👋 <b>"}},"account":"+1"}`
	got := string(AddStyledMessages([]byte(payload)))
	want := `{"envelope":{"timestamp":1700000000000123,"dataMessage":{"timestamp":1700000000000123,"message":"hi 👋 <b>","styledMessage":"hi 👋 <b>"}},"account":"+1"}`
	if got != want {
		t.Errorf("got %s\nwanted %s", got, want)
	}
}
