package client

import (
	"math"
	"testing"
	"time"
)

func TestPollVoteCountIncreasesForLaterVotes(t *testing.T) {
	pollTimestamp := int64(1769271479000)
	first := pollVoteCount(pollTimestamp, time.UnixMilli(pollTimestamp+5_000))
	second := pollVoteCount(pollTimestamp, time.UnixMilli(pollTimestamp+5_300))
	if first < 1 {
		t.Fatalf("expected a positive vote count, got %d", first)
	}
	if second <= first {
		t.Fatalf("expected a later vote to have a higher count, got %d then %d", first, second)
	}
}

func TestPollVoteCountIsAtLeastOne(t *testing.T) {
	pollTimestamp := int64(1769271479000)
	if got := pollVoteCount(pollTimestamp, time.UnixMilli(pollTimestamp)); got != 1 {
		t.Fatalf("expected 1 for a vote at the poll time, got %d", got)
	}
	// clock behind the poll author's clock
	if got := pollVoteCount(pollTimestamp, time.UnixMilli(pollTimestamp-60_000)); got != 1 {
		t.Fatalf("expected 1 for a clock behind the poll time, got %d", got)
	}
}

func TestPollVoteCountIsClamped(t *testing.T) {
	if got := pollVoteCount(0, time.UnixMilli(math.MaxInt64/2)); got != math.MaxInt32 {
		t.Fatalf("expected the count to be clamped to %d, got %d", int32(math.MaxInt32), got)
	}
}
