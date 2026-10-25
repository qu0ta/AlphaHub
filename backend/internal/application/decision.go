package application

import "fmt"

type ReviewRoute string

const (
	RouteReject ReviewRoute = "REJECT"
	RouteManual ReviewRoute = "MANUAL"
	RouteOffer  ReviewRoute = "CREATE_OFFER"
)

func SelectReviewRoute(
	score int,
	lowerThreshold int,
	upperThreshold int,
) (ReviewRoute, error) {
	if score < 0 || score > 100 {
		return "", fmt.Errorf("score must be between 0 and 100")
	}

	if lowerThreshold < 0 ||
		upperThreshold > 100 ||
		lowerThreshold >= upperThreshold {
		return "", fmt.Errorf("invalid scoring thresholds")
	}

	switch {
	case score < lowerThreshold:
		return RouteReject, nil

	case score < upperThreshold:
		return RouteManual, nil

	default:
		return RouteOffer, nil
	}
}
