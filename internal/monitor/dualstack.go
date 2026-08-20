package monitor

// DefaultDualstack is the IP version Mackerel applies to an external monitor
// whose dualstack field was never set. Such a monitor omits the key entirely
// from API responses rather than reporting ipv4, so both sides of a comparison
// have to be widened before they can be compared.
const DefaultDualstack = "ipv4"

// NormalizeDualstack widens an unset dualstack value to its effective meaning.
// Without it the desired side (empty when the CR omits the field) would never
// equal the actual side of a monitor explicitly set to ipv4, and the planner
// would rewrite the monitor on every reconcile.
func NormalizeDualstack(dualstack string) string {
	if dualstack == "" {
		return DefaultDualstack
	}

	return dualstack
}
