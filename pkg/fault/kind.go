package fault

type Kind string

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

func IsFetchOutcome(k Kind) bool {
	_ = k
	return false
}

func Valid(k Kind) bool {
	_ = k
	return false
}
