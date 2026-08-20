# External Monitor `dualstack` Design

## Goal

Mackerel added an IP version setting to external monitors on 2026-04-22. `ExternalMonitorSpec` cannot express it, so a monitor that has to be checked over IPv6 must be managed outside the operator. This design adds `dualstack` to the CRD.

The field was missed by [the previous round of work](2026-08-08-external-monitor-remaining-fields-design.md) because that scoping diffed the CRD against `mackerel-client-go` rather than against the API itself. `mackerel-client-go` v0.47.0 now carries `Dualstack *Dualstack`, so the operator can send it through the existing SDK request path without bypassing or wrapping the client.

## The field

| | |
| --- | --- |
| API parameter | `dualstack` |
| Allowed values | `ipv4`, `ipv6`, `auto` |
| Unset | the key is absent from responses and behaves as `ipv4` |
| Semantics | `ipv4` does not retry over IPv6; `ipv6` does not retry over IPv4; `auto` tries IPv6 first and falls back to IPv4 |

Two properties drive the whole design, both verified against the live API while scoping [#322](https://github.com/SlashNephy/mackerel-operator/issues/322):

1. **A monitor that never set `dualstack` omits the key entirely** rather than reporting `ipv4`. The read side therefore cannot distinguish "unset" from "ipv4" by value alone.
2. **`dualstack` survives a `PUT` that omits it.** This is the opposite of `isMute`, `followRedirect`, `skipCertificateVerification`, and `requestBody`, which reset to their defaults when their key is absent.

[#322](https://github.com/SlashNephy/mackerel-operator/issues/322) additionally reported that `dualstack` is absent from `GET /api/v0/monitors` and appears only on the per-monitor `GET /api/v0/monitors/{id}`. That did not reproduce — see below — so no special handling of list results was needed.

## Where the `ipv4` default is applied

`ipv4` is the effective meaning of an unset value, but it is deliberately **not** materialised in the CRD or the source layer:

- No `+kubebuilder:default=ipv4`. A structural-schema default is applied on read as well as on write, so every existing `ExternalMonitor` would gain `dualstack: ipv4`, that value would reach `DesiredExternalMonitor`, and the desired-state hash of every resource would change — one Update per resource on upgrade.
- `DesiredExternalMonitor.Dualstack` is tagged `omitempty`, so an unset field stays out of the hashed JSON and the hash of a resource that predates the field is unchanged.

The `ipv4` widening lives in the two places that need it:

- `internal/planner` normalises **both** sides before comparing, via `monitor.NormalizeDualstack`. Without it a CR that omits the field would never match a monitor explicitly set to `ipv4`.
- `internal/provider` writes an **explicit** value on every create and update, sending `ipv4` when the CR omits the field.

The provider half is what keeps the loop closed. Because Mackerel preserves `dualstack` across a `PUT` that omits it, omitting the key for an unset CR would leave a monitor set to `ipv6` untouched while the planner — reading that CR as `ipv4` — kept reporting drift, and the same Update would be reissued forever.

This is a deliberate departure from how `maxCheckAttempts` was handled in the previous round, where the source layer widens an unset value to `1` and the resulting hash change was accepted. The difference is that `maxCheckAttempts` is always reported by Mackerel, so the widening has to happen before the hash to keep the two sides comparable at all, whereas `dualstack` can be widened later.

## The list endpoint reports `dualstack` after all

This was checked because it would have mattered: `findActual` falls back to the monitor list when the status carries no monitor ID, and the adoption path never records one. Had the list really omitted `dualstack`, a name-matched, marker-less monitor set to `ipv6` would have been read as `ipv4`, differed from an equally-set CR, and been reported as `OwnershipLost` on every reconcile with no way out.

It reproduces neither through the operator's provider nor through raw HTTP. For a monitor created with `dualstack: "ipv6"`, both endpoints carry the key:

```console
$ curl -sS -H "X-Api-Key: $KEY" "https://api.mackerelio.com/api/v0/monitors/5RpXkkSJ7pA" | jq -c '.monitor | {id, dualstack, hasKey: has("dualstack")}'
{"id":"5RpXkkSJ7pA","dualstack":"ipv6","hasKey":true}

$ curl -sS -H "X-Api-Key: $KEY" "https://api.mackerelio.com/api/v0/monitors" | jq -c '.monitors[] | select(.id=="5RpXkkSJ7pA") | {id, dualstack, hasKey: has("dualstack")}'
{"id":"5RpXkkSJ7pA","dualstack":"ipv6","hasKey":true}
```

The likeliest reading of the original observation is that it was made on a monitor whose `dualstack` was never set. Such a monitor omits the key from *both* endpoints, which looks the same as a list-only omission if the two are not compared on one monitor that has the field set.

## Behaviour change

The operator now owns `dualstack`, so a monitor switched to IPv6 or auto in the web UI is reset to IPv4 on the next reconcile unless the CR declares the value. This is new: previous versions left the field alone, not by design but because they could not express it. It is called out in `README.md` alongside the equivalent warning for `isMute`.
