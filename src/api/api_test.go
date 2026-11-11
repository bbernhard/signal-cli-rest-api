package api

import (
	"encoding/json"
	"testing"
)

func TestUpdateGroupRequestMemberLabel(t *testing.T) {
	var req UpdateGroupRequest
	rawRequest := []byte(`{"member_label":{"name":"Dad","emoji":"\ud83d\udc4b"}}`)

	if err := json.Unmarshal(rawRequest, &req); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if req.MemberLabel == nil {
		t.Fatal("member_label got nil, want value")
	}

	if req.MemberLabel.Name == nil || *req.MemberLabel.Name != "Dad" {
		t.Fatalf("member_label.name got %v, want Dad", req.MemberLabel.Name)
	}

	if req.MemberLabel.Emoji == nil || *req.MemberLabel.Emoji != "\U0001F44B" {
		t.Fatalf("member_label.emoji got %v, want \\U0001F44B", req.MemberLabel.Emoji)
	}
}
