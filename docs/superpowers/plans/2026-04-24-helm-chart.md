# Helm Chart Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a hand-maintained Helm chart for installing `mackerel-operator`.

**Architecture:** Create `charts/mackerel-operator` with standard Helm metadata, values, helpers, templates, and copied CRD. Keep optional metrics and Prometheus resources out of the MVP chart.

**Tech Stack:** Helm v3, Kubernetes YAML, Kubebuilder-generated CRD and RBAC permissions.

---

## Task 1: Add Chart Skeleton

**Files:**
- Create: `charts/mackerel-operator/Chart.yaml`
- Create: `charts/mackerel-operator/values.yaml`
- Create: `charts/mackerel-operator/templates/_helpers.tpl`

- [ ] Create chart metadata with API v2, app version `0.1.0`, and chart version `0.1.0`.
- [ ] Add values for image, replica count, policy, owner ID, hash length, Mackerel API key Secret reference, service account, resources, and security contexts.
- [ ] Add helper templates for chart name, full name, labels, selector labels, and service account name.

## Task 2: Add Runtime Templates

**Files:**
- Create: `charts/mackerel-operator/templates/serviceaccount.yaml`
- Create: `charts/mackerel-operator/templates/rbac.yaml`
- Create: `charts/mackerel-operator/templates/deployment.yaml`

- [ ] Template the ServiceAccount with common labels.
- [ ] Template RBAC for `externalmonitors`, `externalmonitors/status`, `externalmonitors/finalizers`, events, and leader election leases.
- [ ] Template the Deployment with `/manager`, leader election, policy, owner ID, hash length, Mackerel API key Secret env, probes, resources, and restricted security context defaults.

## Task 3: Add CRD and Documentation

**Files:**
- Create: `charts/mackerel-operator/crds/mackerel.starry.blue_externalmonitors.yaml`
- Modify: `README.md`

- [ ] Copy the generated `ExternalMonitor` CRD into chart `crds/`.
- [ ] Add README Helm install example and Secret creation example.

## Task 4: Verify and Commit

**Files:**
- Read: `charts/mackerel-operator`
- Read: `README.md`

- [ ] Run `helm lint charts/mackerel-operator`.
- [ ] Run `helm template mackerel-operator charts/mackerel-operator`.
- [ ] Run `go test ./...`.
- [ ] Commit with `feat: add Helm chart`.
