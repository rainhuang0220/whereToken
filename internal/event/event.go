package event

import "time"

type Quality string

const (
	QualityAuthoritative Quality = "authoritative"
	QualityDegraded      Quality = "degraded"
	QualityEstimated     Quality = "estimated"
	QualityAbsent        Quality = "absent"
)

const (
	DeriveRaw          = "raw"
	DeriveProviderAPI  = "provider_api"
	DeriveDerived      = "derived"
	DeriveDeduplicated = "deduplicated"
	DeriveEstimated    = "estimated"
)

type UsageEvent struct {
	Source, Vendor, SourceRoot                      string
	SessionID, RequestID                            string
	Model, Provider, Workspace                      string
	Timestamp                                       time.Time
	Miss, CacheRead, CacheCreate, Output, Reasoning int64
	Quality                                         Quality
	Derivation                                      string // raw, provider_api, derived, deduplicated, estimated
	SkipRequest                                     bool   // token-only rows (Cursor account API) must not inflate request counts
}

type TurnEvent struct {
	Source, SessionID, Workspace string
	Timestamp                    time.Time
}
