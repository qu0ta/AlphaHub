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
