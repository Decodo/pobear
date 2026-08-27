package fault

// Kind is one of pobear's ten fault kinds.
type Kind string

// Kind values. Auth through Provider are the seven fetch-outcome kinds a
// provider manifest's errors[] may assign; Spec, Parse, and Plugin are
// job-level kinds no manifest may assign.
const (
	Auth      Kind = "auth"
	Network   Kind = "network"
	Timeout   Kind = "timeout"
	HTTP      Kind = "http"
	Blocked   Kind = "blocked"
	RateLimit Kind = "rateLimit"
	Provider  Kind = "provider"
	Spec      Kind = "spec"
	Parse     Kind = "parse"
	Plugin    Kind = "plugin"
)

// IsFetchOutcome reports whether k is one of the seven kinds a provider
// manifest's errors[] may assign.
func IsFetchOutcome(k Kind) bool {
	_ = k
	return false
}

// Valid reports whether k is one of the ten recognized kinds. Stubbed
// until a later phase implements it for real.
func Valid(k Kind) bool {
	_ = k
	return false
}
