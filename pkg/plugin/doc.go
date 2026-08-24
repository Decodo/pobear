// Package plugin implements the `plugin` stage channel: kept independent of
// any one stage implementation since it's meant for whatever plugins the
// project allows creating. Only reachable in `__rpc` mode.
// See docs/JOB_SPEC_REFERENCE.md §6.
package plugin
