# What

<!-- What changes, in one or two sentences. -->

# Why

<!-- The problem this solves. If it fixes a bug, describe the failure: what
     was observed, and what the root cause turned out to be. A reader should
     understand the reasoning without opening the diff. -->

# How it was verified

<!-- Evidence, not intent. Paste the command and its output, name the CI run,
     or describe what was observed in the cluster. "Should work" is not
     verification. -->

- [ ] `go build` and `go test ./...` pass locally or in CI
- [ ] `golangci-lint` is clean
- [ ] Kubernetes changes render: `kubectl kustomize overlays/k3s`
- [ ] Any image change builds for both `linux/amd64` and `linux/arm64`

# Risk

<!-- What breaks if this is wrong, and how to roll it back. Delete if trivial. -->

---

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/):
`feat:`, `fix:`, `chore:`, `refactor:`, `docs:`, `test:`, `ci:`.
