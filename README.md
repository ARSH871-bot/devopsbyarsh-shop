<!-- markdownlint-disable-next-line -->
# devopsbyarsh.shop

**A production-shaped delivery platform built around the OpenTelemetry Astronomy
Shop.** Live at **[devopsbyarsh.shop](https://devopsbyarsh.shop)**.

[![product-catalog-ci](https://github.com/ARSH871-bot/devopsbyarsh-shop/actions/workflows/ci.yaml/badge.svg)](https://github.com/ARSH871-bot/devopsbyarsh-shop/actions/workflows/ci.yaml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg?color=red)](./LICENSE)

## Provenance

The application is **not mine**. It is a fork of
[open-telemetry/opentelemetry-demo](https://github.com/open-telemetry/opentelemetry-demo)
— a 14-service polyglot microservice system (Go, Rust, C#, Java, Kotlin, Python,
C++, PHP, Ruby, Node, TypeScript) — routed here via
[Abhishek Veeramalla's DevOps course](https://github.com/iam-veeramalla), whose
commits remain in `git log` under his name. Credit to both; they wrote the
services and the original single-service pipeline.

**What is mine is everything around it:** the CI/CD, the Kubernetes deployment,
the observability stack, the GitOps layer, and the bugs found and fixed along
the way. That work is listed below, and every item is traceable to a commit.

## What this repository adds

| Area | Upstream / course | Here |
|---|---|---|
| CI | one service, lint non-blocking, no tests | lint-gated, 10 unit tests, multi-arch |
| Images | `linux/amd64` | `linux/amd64` + `linux/arm64`, cross-compiled |
| Image tags | `github.run_id` | commit SHA — idempotent and traceable |
| Toolchain | Go 1.22 (EOL), golangci-lint v1.55 | Go 1.25, golangci-lint v2.12 |
| Vulnerabilities | unscanned | Trivy gate on fixable HIGH/CRITICAL — **0** |
| Provenance | none | SBOM + build provenance attached |
| Signing | none | cosign keyless, verified in CI |
| Actions | mutable tags (`@v4`) | 13/13 pinned to SHAs, Renovate-maintained |
| Token scope | `contents: write` for all jobs | least privilege per job |
| Kubernetes | 19 app Deployments | + collector, Jaeger, Prometheus, Grafana |
| Health | none | gRPC readiness + liveness probes |
| Layout | flat manifests + a duplicate bundle | kustomize base with `k3s` / `eks` overlays |
| CD | none | Argo CD `AppProject` + `Application` |
| Ingress | `host: example.com`, HTTP | real domain, HTTPS, redirect |
| Repo | no protection, no templates | protected `main`, `CODEOWNERS`, PR template |

## Bugs found and fixed

The interesting part. Each was live in the upstream or course version.

**The pipeline was green and shipped a pod that could not serve traffic.**
The manifests were Helm-rendered from chart `1.12.0`, when the service read
`PRODUCT_CATALOG_SERVICE_PORT`. Upstream
[#1864](https://github.com/open-telemetry/opentelemetry-demo/pull/1864) renamed
it to `PRODUCT_CATALOG_PORT` and the manifests were never re-rendered, so they
set a variable nothing reads. `mustMapEnv` would have exited loudly on startup —
except the Dockerfile's `ENV PRODUCT_CATALOG_PORT=8088` satisfied the lookup, so
the container booted on `8088` while the Service targeted `8080`. A crash became
a silent blackhole: `Running`, `1/1 Ready`, zero restarts, every request timing
out.

**CI force-pushed GitHub's internal merge ref onto `main`, three times.**
`git push origin HEAD:main -f` on a `pull_request` event pushes
`refs/pull/N/merge`, not the branch. It landed unreviewed PR content on `main`
and would silently discard anything committed since the PR opened. The evidence
is still in the history as `Merge <sha> into <sha>` commits. Now pushes to the
PR branch, and `main` is protected against force-pushes.

**Kubernetes had no telemetry backend at all.** Every service pointed at
`opentelemetry-demo-otelcol:4317`; no Service by that name existed anywhere in
the repo. Traces, metrics and logs were dropped, and `/grafana` and `/jaeger`
returned 503 — on a project whose entire premise is observability.

**An out-of-support toolchain was silently blocking every security fix.** The
first Trivy run failed with 25 findings, 23 HIGH and 2 CRITICAL. The alpine
base was clean; all of them were Go dependencies. The patched releases required
Go ≥ 1.25, and the project was pinned to Go 1.22 — itself past end of support
and no longer receiving fixes. The individual CVEs were symptoms; the toolchain
pin was the cause. Upgrading it cleared the chain: **25 → 3 → 1 → 0**.

The upgrade then surfaced three things the older toolchain had hidden. Go 1.24
tightened `vet`'s printf analysis and rejected a `status.Errorf` call 1.22 had
accepted. `golangci-lint` v1.55.2 could not compile a `go 1.25.0` module at
all — it exited 3 before inspecting anything, meaning a linter that reported
green while checking nothing. And gRPC 1.83 added `List` to the `HealthServer`
interface, so the CVE fix did not compile until it was implemented.

**cosign signed successfully and produced nothing retrievable.** cosign v3
defaults to publishing signatures through the OCI 1.1 referrers API. Docker Hub
accepts that write and returns success, then does not serve the referrer back,
so verification failed with `no signatures found` against an image that had
just signed cleanly. Without a verify step in CI this would have shipped
green — a pipeline advertising signatures no consumer could check.

**A missing GitHub secret resolves to an empty string, not an error.** The
workflow referenced `DOCKER_USERNAME`; the configured secret was
`DOCKERHUB_USERNAME`. Docker login failed with `Username required` rather than
anything pointing at the cause.

**`go test ./...` passed against zero test files.** The unit-test stage was
decorative. Now 9 tests covering every gRPC handler and the catalog loader —
including a duplicate-ID check, because `GetProduct` breaks on first match, so a
duplicate would make one product permanently unreachable.

**Shell scripts were checked out with CRLF.** `.gitattributes` pinned `eol=lf`
for `gradlew` alone, so `docker-gen-proto.sh`, `ide-gen-proto.sh` and `run.bash`
got `#!/bin/sh\r` on Windows — `bad interpreter: ^M` in any Linux shell or
container.

Also: Docker build cache mounts attached to a bare `mkdir` instead of to
`go mod download` and `go build`, so every build recompiled the full dependency
tree; a `20Mi` memory limit with no `GOMEMLIMIT`, which OOMKills a Go binary
carrying the OTel SDK; and `complete-deploy.yaml`, a byte-for-byte duplicate of
the per-service manifests that makes Argo CD refuse to sync.

## Why k3s and not EKS

EKS was built first and is kept in `overlays/eks`, but it does not run the site.
The control plane alone bills **USD $0.10/hour flat — roughly $120 NZD/month —
whether or not a single pod is scheduled**, before ~$50 for a node and ~$30 for
an ALB. That is ~$200 NZD/month for a portfolio demo.

The live deployment runs k3s on an Oracle Cloud Always Free ARM instance
(4 cores / 24 GB, permanently $0). Both paths are in the repository:

```
kubernetes/          platform-neutral kustomize base
overlays/k3s/        Traefik ingress, cert-manager + Let's Encrypt   <- live
overlays/eks/        ALB ingress, ACM certificate discovery
```

This is also why images are built for `arm64`: Ampere A1 is ARM, and an
amd64-only image fails there with `exec format error` after a pull that appears
to succeed.

## Pipeline

```
PR to main
  ├─ build         go build + go test -v ./...
  ├─ code-quality  golangci-lint                      (gates the image)
  ├─ docker        buildx -> amd64 + arm64 -> Docker Hub
  │                  ├─ SBOM + provenance attached
  │                  ├─ Trivy: fail on fixable HIGH/CRITICAL
  │                  ├─ cosign sign   (keyless, by digest)
  │                  └─ cosign verify (asserts repo + workflow identity)
  └─ updatek8s     write the tag into the manifest, commit to the PR branch
                            |
                     merge to main
                            |
                   Argo CD syncs overlays/k3s
```

Images are tagged with the **commit SHA**, never `latest`. `run_id` was tried
first and rejected: it changes on every run, including a re-run of unchanged
code, so the manifest was rewritten and committed each time. A content-derived
tag makes the step idempotent, and `git show <tag>` resolves the exact source
of whatever is running in the cluster.

Signing runs after scanning and signs the **digest**, not the tag — a tag can
be repointed at different content, a digest cannot.

## Verifying a published image

Every image carries a cosign signature proving which repository and workflow
built it:

```bash
cosign verify docker.io/arsh885/product-catalog:<commit-sha> \
  --certificate-identity-regexp "^https://github.com/ARSH871-bot/devopsbyarsh-shop/\.github/workflows/" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com"
```

Keyless, so there is no public key to distribute or private key to rotate — the
signing identity is the workflow itself, recorded in the Sigstore transparency
log.

## Running it

```bash
docker compose up --force-recreate --remove-orphans --detach   # full stack
make start-minimal                                             # without Kafka/Postgres

kubectl kustomize overlays/k3s     # render the live deployment
kubectl kustomize overlays/eks     # render the AWS path
```

---

Everything below is the upstream OpenTelemetry Demo documentation, retained as-is.

<!-- markdownlint-disable-next-line -->
# <img src="https://opentelemetry.io/img/logos/opentelemetry-logo-nav.png" alt="OTel logo" width="45"> OpenTelemetry Demo

[![Slack](https://img.shields.io/badge/slack-@cncf/otel/demo-brightgreen.svg?logo=slack)](https://cloud-native.slack.com/archives/C03B4CWV4DA)
[![Version](https://img.shields.io/github/v/release/open-telemetry/opentelemetry-demo?color=blueviolet)](https://github.com/open-telemetry/opentelemetry-demo/releases)
[![Commits](https://img.shields.io/github/commits-since/open-telemetry/opentelemetry-demo/latest?color=ff69b4&include_prereleases)](https://github.com/open-telemetry/opentelemetry-demo/graphs/commit-activity)
[![Downloads](https://img.shields.io/docker/pulls/otel/demo)](https://hub.docker.com/r/otel/demo)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg?color=red)](https://github.com/open-telemetry/opentelemetry-demo/blob/main/LICENSE)
[![Integration Tests](https://github.com/open-telemetry/opentelemetry-demo/actions/workflows/run-integration-tests.yml/badge.svg)](https://github.com/open-telemetry/opentelemetry-demo/actions/workflows/run-integration-tests.yml)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/opentelemetry-demo)](https://artifacthub.io/packages/helm/opentelemetry-helm/opentelemetry-demo)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/9247/badge)](https://www.bestpractices.dev/en/projects/9247)

## Welcome to the OpenTelemetry Astronomy Shop Demo

This repository contains the OpenTelemetry Astronomy Shop, a microservice-based
distributed system intended to illustrate the implementation of OpenTelemetry in
a near real-world environment.

Our goals are threefold:

- Provide a realistic example of a distributed system that can be used to
  demonstrate OpenTelemetry instrumentation and observability.
- Build a base for vendors, tooling authors, and others to extend and
  demonstrate their OpenTelemetry integrations.
- Create a living example for OpenTelemetry contributors to use for testing new
  versions of the API, SDK, and other components or enhancements.

We've already made [huge
progress](https://github.com/open-telemetry/opentelemetry-demo/blob/main/CHANGELOG.md),
and development is ongoing. We hope to represent the full feature set of
OpenTelemetry across its languages in the future.

If you'd like to help (**which we would love**), check out our [contributing
guidance](./CONTRIBUTING.md).

If you'd like to extend this demo or maintain a fork of it, read our
[fork guidance](https://opentelemetry.io/docs/demo/forking/).

## Quick start

You can be up and running with the demo in a few minutes. Check out the docs for
your preferred deployment method:

- [Docker](https://opentelemetry.io/docs/demo/docker_deployment/)
- [Kubernetes](https://opentelemetry.io/docs/demo/kubernetes_deployment/)

## Documentation

For detailed documentation, see [Demo Documentation][docs]. If you're curious
about a specific feature, the [docs landing page][docs] can point you in the
right direction.

## Demos featuring the Astronomy Shop

We welcome any vendor to fork the project to demonstrate their services and
adding a link below. The community is committed to maintaining the project and
keeping it up to date for you.

|                           |                |                                  |
|---------------------------|----------------|----------------------------------|
| [AlibabaCloud LogService] | [Elastic]      | [OpenSearch]                     |
| [AppDynamics]             | [Google Cloud] | [Sentry]                         |
| [Aspecto]                 | [Grafana Labs] | [ServiceNow Cloud Observability] |
| [Axiom]                   | [Guance]       | [Splunk]                         |
| [Axoflow]                 | [Honeycomb.io] | [Sumo Logic]                     |
| [Azure Data Explorer]     | [Instana]      | [TelemetryHub]                   |
| [Coralogix]               | [Kloudfuse]    | [Teletrace]                      |
| [Dash0]                   | [Liatrio]      | [Tracetest]                      |
| [Datadog]                 | [Logz.io]      | [Uptrace]                        |
| [Dynatrace]               | [New Relic]    |                                  |

## Contributing

To get involved with the project see our [CONTRIBUTING](CONTRIBUTING.md)
documentation. Our [SIG Calls](CONTRIBUTING.md#join-a-sig-call) are every other
Monday at 8:30 AM PST and anyone is welcome.

## Project leadership

[Maintainers](https://github.com/open-telemetry/community/blob/main/guides/contributor/membership.md#maintainer)
([@open-telemetry/demo-maintainers](https://github.com/orgs/open-telemetry/teams/demo-maintainers)):

- [Juliano Costa](https://github.com/julianocosta89), Datadog
- [Mikko Viitanen](https://github.com/mviitane), Dynatrace
- [Pierre Tessier](https://github.com/puckpuck), Honeycomb

[Approvers](https://github.com/open-telemetry/community/blob/main/guides/contributor/membership.md#approver)
([@open-telemetry/demo-approvers](https://github.com/orgs/open-telemetry/teams/demo-approvers)):

- [Cedric Ziel](https://github.com/cedricziel) Grafana Labs
- [Penghan Wang](https://github.com/wph95), AppDynamics
- [Reiley Yang](https://github.com/reyang), Microsoft
- [Roger Coll](https://github.com/rogercoll), Elastic
- [Ziqi Zhao](https://github.com/fatsheep9146), Alibaba

Emeritus:

- [Austin Parker](https://github.com/austinlparker)
- [Carter Socha](https://github.com/cartersocha)
- [Michael Maxwell](https://github.com/mic-max)
- [Morgan McLean](https://github.com/mtwo)

### Thanks to all the people who have contributed

[![contributors](https://contributors-img.web.app/image?repo=open-telemetry/opentelemetry-demo)](https://github.com/open-telemetry/opentelemetry-demo/graphs/contributors)

[docs]: https://opentelemetry.io/docs/demo/

<!-- Links for Demos featuring the Astronomy Shop section -->

[AlibabaCloud LogService]: https://github.com/aliyun-sls/opentelemetry-demo
[AppDynamics]: https://www.appdynamics.com/blog/cloud/how-to-observe-opentelemetry-demo-app-in-appdynamics-cloud/
[Aspecto]: https://github.com/aspecto-io/opentelemetry-demo
[Axiom]: https://play.axiom.co/axiom-play-qf1k/dashboards/otel.traces.otel-demo-traces
[Axoflow]: https://axoflow.com/opentelemetry-support-in-more-detail-in-axosyslog-and-syslog-ng/
[Azure Data Explorer]: https://github.com/Azure/Azure-kusto-opentelemetry-demo
[Coralogix]: https://coralogix.com/blog/configure-otel-demo-send-telemetry-data-coralogix
[Dash0]: https://github.com/dash0hq/opentelemetry-demo
[Datadog]: https://docs.datadoghq.com/opentelemetry/guide/otel_demo_to_datadog
[Dynatrace]: https://www.dynatrace.com/news/blog/opentelemetry-demo-application-with-dynatrace/
[Elastic]: https://github.com/elastic/opentelemetry-demo
[Google Cloud]: https://github.com/GoogleCloudPlatform/opentelemetry-demo
[Grafana Labs]: https://github.com/grafana/opentelemetry-demo
[Guance]: https://github.com/GuanceCloud/opentelemetry-demo
[Honeycomb.io]: https://github.com/honeycombio/opentelemetry-demo
[Instana]: https://github.com/instana/opentelemetry-demo
[Kloudfuse]: https://github.com/kloudfuse/opentelemetry-demo
[Liatrio]: https://github.com/liatrio/opentelemetry-demo
[Logz.io]: https://logz.io/learn/how-to-run-opentelemetry-demo-with-logz-io/
[New Relic]: https://github.com/newrelic/opentelemetry-demo
[OpenSearch]: https://github.com/opensearch-project/opentelemetry-demo
[Sentry]: https://github.com/getsentry/opentelemetry-demo
[ServiceNow Cloud Observability]: https://docs.lightstep.com/otel/quick-start-operator#send-data-from-the-opentelemetry-demo
[Splunk]: https://github.com/signalfx/opentelemetry-demo
[Sumo Logic]: https://www.sumologic.com/blog/common-opentelemetry-demo-application/
[TelemetryHub]: https://github.com/TelemetryHub/opentelemetry-demo/tree/telemetryhub-backend
[Teletrace]: https://github.com/teletrace/opentelemetry-demo
[Tracetest]: https://github.com/kubeshop/opentelemetry-demo
[Uptrace]: https://github.com/uptrace/uptrace/tree/master/example/opentelemetry-demo
