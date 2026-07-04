package domain

import "testing"

func TestRiderIDHappyPath(t *testing.T) {
	var id RiderID = "rider-abc-123"
	if id != "rider-abc-123" {
		t.Errorf("expected rider-abc-123, got %q", id)
	}
}

func TestDriverIDHappyPath(t *testing.T) {
	var id DriverID = "driver-xyz-456"
	if id != "driver-xyz-456" {
		t.Errorf("expected driver-xyz-456, got %q", id)
	}
}

func TestTripIDHappyPath(t *testing.T) {
	var id TripID = "trip-789"
	if id != "trip-789" {
		t.Errorf("expected trip-789, got %q", id)
	}
}

// Type aliases (=) are fully interchangeable with string — verify no conversion needed.
func TestIDsAreStringAliases(t *testing.T) {
	rider := RiderID("rider-1")
	var s string = rider
	if s != "rider-1" {
		t.Errorf("RiderID not directly assignable to string: got %q", s)
	}

	driver := DriverID("driver-2")
	s = driver
	if s != "driver-2" {
		t.Errorf("DriverID not directly assignable to string: got %q", s)
	}

	trip := TripID("trip-3")
	s = trip
	if s != "trip-3" {
		t.Errorf("TripID not directly assignable to string: got %q", s)
	}
}

// Edge case: zero value must be empty string, not some sentinel.
func TestIDZeroValues(t *testing.T) {
	var r RiderID
	var d DriverID
	var trip TripID
	if r != "" {
		t.Errorf("RiderID zero value: want \"\", got %q", r)
	}
	if d != "" {
		t.Errorf("DriverID zero value: want \"\", got %q", d)
	}
	if trip != "" {
		t.Errorf("TripID zero value: want \"\", got %q", trip)
	}
}
