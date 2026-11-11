package client

import (
	"encoding/json"
	"testing"
)

func TestSignalCliGroupEntryMemberLabelFields(t *testing.T) {
	var signalCliGroupEntry SignalCliGroupEntry
	rawGroup := []byte(`{"id":"abc","members":[{"number":"+1234","uuid":"uuid-1","labelEmoji":"\ud83d\udc4b","label":"Dad"}]}`)

	if err := json.Unmarshal(rawGroup, &signalCliGroupEntry); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	memberLabel, memberLabelEmoji := findOwnGroupMemberLabel(signalCliGroupEntry.Members, "+1234")

	if memberLabel != "Dad" {
		t.Fatalf("memberLabel got %v, want Dad", memberLabel)
	}

	if memberLabelEmoji != "\U0001F44B" {
		t.Fatalf("memberLabelEmoji got %v, want \\U0001F44B", memberLabelEmoji)
	}

	groupEntry := GroupEntry{
		MemberLabel:      memberLabel,
		MemberLabelEmoji: memberLabelEmoji,
	}

	encodedGroup, err := json.Marshal(groupEntry)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal(encodedGroup, &response); err != nil {
		t.Fatalf("json.Unmarshal() response error: %v", err)
	}

	if response["member_label"] != "Dad" {
		t.Fatalf("member_label got %v, want Dad", response["member_label"])
	}

	if response["member_label_emoji"] != "\U0001F44B" {
		t.Fatalf("member_label_emoji got %v, want \\U0001F44B", response["member_label_emoji"])
	}
}

func TestSignalCliGroupEntryMissingMemberLabelFields(t *testing.T) {
	var signalCliGroupEntry SignalCliGroupEntry
	rawGroup := []byte(`{"id":"abc","members":[{"number":"+1234","uuid":"uuid-1"}]}`)

	if err := json.Unmarshal(rawGroup, &signalCliGroupEntry); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	memberLabel, memberLabelEmoji := findOwnGroupMemberLabel(signalCliGroupEntry.Members, "+1234")

	groupEntry := GroupEntry{
		MemberLabel:      memberLabel,
		MemberLabelEmoji: memberLabelEmoji,
	}

	encodedGroup, err := json.Marshal(groupEntry)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal(encodedGroup, &response); err != nil {
		t.Fatalf("json.Unmarshal() response error: %v", err)
	}

	if response["member_label"] != "" {
		t.Fatalf("member_label got %v, want empty string", response["member_label"])
	}

	if response["member_label_emoji"] != "" {
		t.Fatalf("member_label_emoji got %v, want empty string", response["member_label_emoji"])
	}
}
