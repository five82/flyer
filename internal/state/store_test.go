package state

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/five82/flyer/internal/spindle"
)

func TestStore_UpdateAndSnapshotClone(t *testing.T) {
	var s Store

	status := &spindle.StatusResponse{Running: true, PID: 123}
	queue := []spindle.QueueItem{{ID: 1}, {ID: 2}}

	before := time.Now()
	s.Update(status, queue, nil)

	snap := s.Snapshot()
	if !snap.HasStatus || snap.Status.PID != 123 {
		t.Fatalf("snapshot status = %#v, want pid=123 HasStatus=true", snap.Status)
	}
	if len(snap.Queue) != 2 || snap.Queue[0].ID != 1 {
		t.Fatalf("snapshot queue = %#v, want 2 items", snap.Queue)
	}
	if snap.LastUpdated.Before(before) {
		t.Fatalf("LastUpdated = %v, want >= %v", snap.LastUpdated, before)
	}
	if snap.LastError != nil {
		t.Fatalf("LastError = %v, want nil", snap.LastError)
	}

	// Returned snapshot should be independent of the stored one.
	snap.Queue[0].ID = 999
	snap2 := s.Snapshot()
	if snap2.Queue[0].ID != 1 {
		t.Fatalf("Snapshot should clone queue; got id %d want 1", snap2.Queue[0].ID)
	}
}

func TestStore_UpdateErrorKeepsPreviousData(t *testing.T) {
	var s Store

	s.Update(&spindle.StatusResponse{PID: 1}, []spindle.QueueItem{{ID: 1}}, nil)
	prev := s.Snapshot()

	before := time.Now()
	origErr := errors.New("boom")
	s.Update(nil, nil, origErr)

	snap := s.Snapshot()
	if snap.HasStatus != prev.HasStatus || snap.Status.PID != prev.Status.PID {
		t.Fatalf("status changed on error: got %#v want %#v", snap.Status, prev.Status)
	}
	if len(snap.Queue) != 1 || snap.Queue[0].ID != 1 {
		t.Fatalf("queue changed on error: got %#v want %#v", snap.Queue, prev.Queue)
	}
	if snap.LastUpdated.Before(before) {
		t.Fatalf("LastUpdated = %v, want >= %v", snap.LastUpdated, before)
	}
	if snap.LastError == nil || snap.LastError.Error() != "boom" {
		t.Fatalf("LastError = %v, want boom", snap.LastError)
	}
	if reflect.ValueOf(snap.LastError).Pointer() == reflect.ValueOf(origErr).Pointer() {
		t.Fatalf("Snapshot should clone error instance")
	}
}
