package bathing

// SupplementClass is one annual class from a second source.
type SupplementClass struct {
	SiteID  string
	Season  int
	Quality string // stored key, as in qualities
}

// Supplement is a validated snapshot that fills class gaps. A nil Supplement
// means no fill. The datahub package builds it; this package cannot import
// datahub because datahub imports this one.
type Supplement struct {
	Edition   string
	Published string
	URL       string
	Classes   []SupplementClass
}
