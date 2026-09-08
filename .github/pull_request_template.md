## What

<!-- What does this change do? -->

## Why

<!-- What problem does it solve? Link the issue if there is one. -->

Closes #

## How to verify

<!-- The steps a reviewer should run. -->

```bash
make ci
```

## Checklist

- [ ] PR title follows [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `chore:` …) — it becomes the squash-merge message
- [ ] Tests cover the change
- [ ] `make ci` passes locally
- [ ] Docs / README updated if behaviour changed
- [ ] Targets the right branch (`develop` for features, `staging` for a release candidate, `main` for a hotfix)
