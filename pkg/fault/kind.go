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
	switch k {
	case Auth, Network, Timeout, HTTP, Blocked, RateLimit, Provider:
		return true
	default:
		return false
	}
}

func Valid(k Kind) bool {
	switch k {
	case Auth, Network, Timeout, HTTP, Blocked, RateLimit, Provider, Spec, Parse, Plugin:
		return true
	default:
		return false
	}
}
