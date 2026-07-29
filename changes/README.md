# CLI change fragments

Every pull request that changes `cli/**` must add a Changie YAML fragment under
`unreleased/`. Changie aggregates these fragments, selects the highest required
semantic-version bump, and renders the versioned release notes and cumulative
changelog.

For a user-visible change:

```sh
cd cli
changie new --kind Fixed \
  --body "Preserve the login link when the terminal is resized."
```

Kinds determine the bump automatically:

- `Fixed`, `Changed`, or `Security`: patch
- `Added` or `Deprecated`: minor
- `Breaking` or `Removed`: major
- `Internal`: no release bump

Use an explicit internal fragment when the change has no shipped user impact:

```sh
cd cli
changie new --kind Internal \
  --body "Adds test coverage without changing shipped behavior."
```

Bodies must describe observable user impact, not implementation details.
Ordinary pull requests must not edit `cli/VERSION`; the release-preparation
command owns version selection.
