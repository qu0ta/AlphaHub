package application

import "testing"

func TestSelectReviewRoute(t *testing.T) {
	tests := []struct {
		score int
		want  ReviewRoute
	}{
		{0, RouteReject},
		{39, RouteReject},
		{40, RouteManual},
		{67, RouteManual},
		{69, RouteManual},
		{70, RouteOffer},
		{100, RouteOffer},
	}
	for _, tt := range tests {
		got, err := SelectReviewRoute(tt.score, 40, 70)
		if err != nil {
			t.Fatalf("score=%d: %v", tt.score, err)
		}

		if got != tt.want {
			t.Errorf(
				"score=%d: got %s, want %s",
				tt.score,
				got,
				tt.want,
			)
		}
	}
}
func TestSelectReviewRouteInvalidInput(t *testing.T) {
	cases := []struct {
		score int
		lower int
		upper int
	}{
		{-1, 40, 70},
		{101, 40, 70},
		{50, 70, 40},
		{50, 40, 40},
		{50, -1, 70},
		{50, 40, 101},
	}

	for _, tc := range cases {
		_, err := SelectReviewRoute(tc.score, tc.lower, tc.upper)
		if err == nil {
			t.Errorf("expected error for %+v", tc)
		}
	}
}

func TestCalculateMonthlyPayment(t *testing.T) {
	for _, tc := range []struct {
		amount       int64
		months, rate int
		want         int64
	}{
		{100, 3, 0, 34}, {100000, 1, 1200, 101000}, {10000000, 12, 1200, 888488},
	} {
		got, err := CalculateMonthlyPayment(tc.amount, tc.months, tc.rate)
		if err != nil || got != tc.want {
			t.Fatalf("payment %+v: got=%d err=%v", tc, got, err)
		}
	}
	if _, err := CalculateMonthlyPayment(9223372036854775807, 1, 1200); err == nil {
		t.Fatal("overflow accepted")
	}
	if _, err := CalculateMonthlyPayment(100, 601, 0); err == nil {
		t.Fatal("excessive term accepted")
	}
}
