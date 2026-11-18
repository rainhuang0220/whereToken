package cursor

import (
	"errors"
	"net/http"

	"github.com/rainhuang0220/whereToken/internal/adapter"
)

var errNetwork = usageKind("NETWORK_FAILURE")
var errCoverage = usageKind("UNVERIFIED_COVERAGE")

// CollectionFailure maps a collection error to a diagnostic code. It never
// returns provider text, paths, credentials or response bodies.
func CollectionFailure(err error) string {
	if err == nil {
		return ""
	}
	var status adapter.ErrStatus
	if errors.As(err, &status) {
		switch status.Code {
		case http.StatusUnauthorized, http.StatusForbidden:
			return "AUTH"
		case http.StatusTooManyRequests:
			return "RATE_LIMIT"
		default:
			return "SOURCE_UNAVAILABLE"
		}
	}
	switch {
	case errors.Is(err, errOSPermission), errors.Is(err, errSealedCredential),
		errors.Is(err, errTokenRejected), errors.Is(err, errNoLocalAuth),
		errors.Is(err, errAccountUsageUnavailable), errors.Is(err, errAccountMismatch),
		errors.Is(err, errUnboundHistory):
		return "AUTH"
	case errors.Is(err, errRateLimited):
		return "RATE_LIMIT"
	case errors.Is(err, errNetwork):
		return "NETWORK"
	case errors.Is(err, errPaginationIncomplete), errors.Is(err, errUsagePageLimit),
		errors.Is(err, errBillingAvailability), errors.Is(err, errCoverage):
		return "COVERAGE"
	case errors.Is(err, errSchemaChanged):
		return "SCHEMA"
	default:
		return "SOURCE_UNAVAILABLE"
	}
}
