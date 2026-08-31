// Package fault defines the fault-kind taxonomy (auth, network, timeout,
// http, blocked, rateLimit, provider, plus the job-level spec/parse/plugin
// kinds): what a provider manifest's errors[] classifies a response into,
// and what the CLI's exit codes key off of. It does not gate the fetch
// engine's retry decision — every failed attempt is retried up to
// limits.retries regardless of kind. See docs/PROVIDER_REFERENCE.md §10.
package fault
