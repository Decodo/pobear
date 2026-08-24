// Package fetch dispatches a job's fetch to whichever tier fetch.use
// selects (local, proxy, or provider), and owns cross-tier concerns:
// concurrency, rate limiting, and retry escalation between tiers.
package fetch
