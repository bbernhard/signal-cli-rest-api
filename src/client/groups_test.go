package client

import (
	"encoding/json"
	"testing"
)

// sampleListGroupsJSON mirrors the shape of signal-cli's `listGroups` JSON output
// (see signal-cli's ListGroupsCommand / the jsonrpc man page). It contains a group
// the account is still in (isMember=true), a group it has left or been removed from
// (isMember=false) — the "ghost" group that the REST API previously could not
// distinguish — and a blocked-but-still-member group to verify the two flags are
// mapped independently.
const sampleListGroupsJSON = `[
  {
    "id": "Pmpi+EfPWmsxiomLe9Nx2XF9HOE483p6iKiFj65iMwI=",
    "name": "Current Group",
    "description": "still a member",
    "isMember": true,
    "isBlocked": false,
    "members": [{"number": "+15551230001", "uuid": "11111111-1111-1111-1111-111111111111"}],
    "pendingMembers": [],
    "requestingMembers": [],
    "admins": [{"number": "+15551230001", "uuid": "11111111-1111-1111-1111-111111111111"}],
    "groupInviteLink": "",
    "permissionAddMember": "EVERY_MEMBER",
    "permissionSendMessage": "EVERY_MEMBER"
  },
  {
    "id": "Zm9vYmFyYmF6cXV4MTIzNDU2Nzg5MGFiY2RlZmdoaWo=",
    "name": "Left Group",
    "description": "removed or left",
    "isMember": false,
    "isBlocked": false,
    "members": [],
    "pendingMembers": [],
    "requestingMembers": [],
    "admins": [],
    "groupInviteLink": ""
  },
  {
    "id": "YmxvY2tlZGdyb3VwaWQwMDAwMDAwMDAwMDAwMDAwMDA=",
    "name": "Blocked But Member",
    "description": "blocked yet still a member",
    "isMember": true,
    "isBlocked": true,
    "members": [],
    "pendingMembers": [],
    "requestingMembers": [],
    "admins": [],
    "groupInviteLink": ""
  }
]`

// TestSignalCliGroupEntryToExpandedGroupEntry verifies that the signal-cli isMember
// flag is carried through to the REST ExpandedGroupEntry.Member field, independently
// of the isBlocked -> Blocked mapping.
func TestSignalCliGroupEntryToExpandedGroupEntry(t *testing.T) {
	var entries []SignalCliGroupEntry
	if err := json.Unmarshal([]byte(sampleListGroupsJSON), &entries); err != nil {
		t.Fatalf("failed to unmarshal sample listGroups JSON: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 group entries, got %d", len(entries))
	}

	cases := []struct {
		name        string
		wantMember  bool
		wantBlocked bool
	}{
		{"Current Group", true, false},
		{"Left Group", false, false},
		{"Blocked But Member", true, true},
	}

	for i, c := range cases {
		got := signalCliGroupEntryToExpandedGroupEntry(entries[i])

		if got.Name != c.name {
			t.Errorf("entry %d: Name = %q, want %q", i, got.Name, c.name)
		}
		if got.Member != c.wantMember {
			t.Errorf("%s: Member = %v, want %v (must reflect signal-cli isMember)", c.name, got.Member, c.wantMember)
		}
		if got.Blocked != c.wantBlocked {
			t.Errorf("%s: Blocked = %v, want %v", c.name, got.Blocked, c.wantBlocked)
		}
		if got.InternalId != entries[i].Id {
			t.Errorf("%s: InternalId = %q, want %q", c.name, got.InternalId, entries[i].Id)
		}
	}
}
