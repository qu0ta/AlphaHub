package application

import (
	"fmt"
	"math/big"
	"time"
)

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

const AutomaticOfferValidity = 7 * 24 * time.Hour

func CalculateMonthlyPayment(amount int64, months, annualRateBps int) (int64, error) {
	if amount <= 0 || months < 1 || months > 600 || annualRateBps < 0 || int64(annualRateBps) > 2147483647 {
		return 0, ErrInvalidInput
	}
	p := big.NewInt(amount)
	var numerator, denominator *big.Int
	if annualRateBps == 0 {
		numerator = p
		denominator = big.NewInt(int64(months))
	} else {
		base := big.NewInt(120000 + int64(annualRateBps))
		power := new(big.Int).Exp(base, big.NewInt(int64(months)), nil)
		denominatorPower := new(big.Int).Exp(big.NewInt(120000), big.NewInt(int64(months)), nil)

		numerator = new(big.Int).Mul(p, big.NewInt(int64(annualRateBps)))
		numerator.Mul(numerator, power)

		denominator = new(big.Int).Sub(power, denominatorPower)
		denominator.Mul(denominator, big.NewInt(120000))
	}

	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)

	if remainder.Sign() > 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, ErrInvalidInput
	}
	return quotient.Int64(), nil
}
