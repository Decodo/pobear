// Package fault defines the fault-kind taxonomy (auth, network, timeout,
// http, blocked, rateLimit, provider, plus the job-level spec/parse/plugin
// kinds): what a provider manifest's errors[] classifies a response into,
// and what the fetch engine's retry-escalation and the CLI's exit codes
// both key off of. See docs/PROVIDER_REFERENCE.md §10.
package fault
