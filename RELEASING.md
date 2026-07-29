# Releasing Thunder CLI

CLI releases use [Changie](https://changie.dev/) fragments to produce semantic
versions and user-facing release notes. Run this process from the `thundernetes`
monorepo; the public `thunder-cli` repository is an exported release target.

Install the tool locally with:

```sh
brew install changie
```

## Pull requests

Every pull request that changes `cli/**` must add a fragment under
`cli/changes/unreleased/`. Use the `Internal` kind only for changes with no
shipped user impact. The same rule applies to the CLI release workflows and
Changie configuration. CI validates the fragments and the policy.

Do not update `cli/VERSION` in an ordinary pull request.

## Prepare a release

From the monorepo root, run:

```sh
make prepare-cli-release
```

This command:

1. Validates all pending fragments.
2. Runs `changie batch auto` to select the highest required SemVer bump.
3. Updates `cli/VERSION`.
4. Generates versioned notes under `cli/changes/`.
5. Regenerates `cli/CHANGELOG.md`.
6. Consumes the pending fragments.

Review the generated notes and open a release-preparation pull request. That PR
must contain only the generated release metadata.

## Publish

After the release-preparation pull request is merged, run:

```sh
make deploy-cli
```

Deployment verifies that the version, current release notes, and changelog agree
before building or publishing. GitHub receives the versioned Changie notes; it
no longer attempts to infer notes from the exported subtree history.
