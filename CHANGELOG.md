# Changelog

All notable Thunder CLI changes are recorded here. Releases before this
change-fragment process was adopted remain available on the
[GitHub releases page](https://github.com/Thunder-Compute/thunder-cli/releases).


## v2.3.0 - 2026-11-05

### Added
- Show instance image build progress with a 15-minute estimate while creation continues in the background.

### Fixed
- tnr status now refreshes promptly when an instance moves to its next state and while instances are stopping or snapshotting, and shows elapsed time once an operation runs past its estimate.

**Full Changelog**: https://github.com/Thunder-Compute/thunder-cli/compare/v2.2.0...v2.3.0

## v2.2.0 - 2026-10-27

### Added
- Send bug reports and product feedback to Thunder support with tnr report.

### Fixed
- Use ubuntu and the selected private key in generated SSH commands, and bypass local SSH agents for manual connections and file transfers to prevent hangs.
- Restore the inline terminal interface from v2.0.71 for instance creation, status monitoring, and loading indicators, keeping status changes in one live table.
- Cancel menus and loading indicators quietly, handle keyboard and signal interruptions consistently, and wait for submitted updates to report their actual result.
- Close interrupted SSH sessions reliably and clear connection progress when cancelled. Interrupted file transfers now exit with code 130 and warn that destination files may be incomplete.
- Allow uppercase Q to cancel instance creation at the disk-size step.
- Explain self-update permission failures with sudo's diagnostic and recovery steps, avoid password prompts without an interactive terminal, and honor the update timeout during installation.
- Exit quietly when choosing No, Cancel in instance and snapshot deletion menus; use Esc to return to selection.
- Classify released CLI error reports as production or other based on the API destination, preserve dev for development builds, and omit the API URL tag from reports.
- Keep JSON command output free of terminal control sequences during startup.

<!-- Internal changes -->
<!-- Add an opt-in local preview for deletion menus using sample data and disabled deletion requests. -->
<!-- Separate authenticated HTTP transport from ordinary API and support-report response handling. -->
<!-- Add a real-rsync regression test for SSH key paths with spaces and quotes (ported from thunder-cli PR 55). -->
<!-- Restore the missing Sentry import so the CLI builds after the environment classification change. -->

**Full Changelog**: https://github.com/Thunder-Compute/thunder-cli/compare/v2.1.0...v2.2.0

## v2.1.0 - 2026-09-16

### Added
- Sign in with OAuth through your browser using tnr login, with automatic credential refresh for continued CLI access.

### Fixed
- Keep saved credentials and avoid opening browser login when validation fails because of a temporary API, network, or OAuth provider error.
- Start a new browser login when a saved CLI refresh token has expired, was already used, or no longer exists.
- Prevent background subscription checks from interrupting browser-authenticated CLI sessions during token refresh.
- Prevent login from reporting success when an invalid TNR_API_TOKEN would continue overriding newly saved credentials.
- Allow login to recover after CLI access is revoked in Connected Apps.
- Prevent simultaneous browser and manual login completions from mixing credential sources.
- Clarify how to clear local credentials and check member-wide CLI access when logout revocation cannot authenticate.
- Preserve refreshed browser login credentials when a command times out or is cancelled during token renewal.
- Keep background subscription checks on the trusted API and allow login to replace invalid saved credentials, including with --token --json.
- Show how to sign in again or replace a rejected API token when a command fails authentication, including guidance for TNR_API_TOKEN overrides.
- Use the selected API environment's console token page for manual and headless login.
- Prevent file transfers from hanging on an unrelated SSH agent by using only the saved instance key.

### Security
- Use dedicated OAuth credentials for browser login, rotate credentials automatically, and let logout securely revoke the current member's Thunder CLI access without signing out Console or affecting other organization members.
- Stop copying the local CLI authentication credential onto instances during connect.
- Prevent a running command from silently switching accounts when another CLI process logs out and signs in again.
- Prevent project configuration and API URL overrides from sending browser login credentials to a different API origin.

<!-- Internal changes -->
<!-- Keep authentication tests focused on credential safety and remove redundant coverage. -->
<!-- Add localhost and staging CLI runners with isolated credentials for rollout testing. -->
<!-- Pin GoReleaser to v2.17.1 to preserve manual CLI releases without a version tag in the monorepo checkout. -->

**Full Changelog**: https://github.com/Thunder-Compute/thunder-cli/compare/v2.0.72...v2.1.0

## v2.0.72 - 2026-09-15

### Added
- Sign in from SSH and headless machines with automatic manual-login instructions and --manual. Paste a token or callback URL, and retry errors without restarting login.

### Fixed
- Keep shared command progress and monitored status stable across terminal resizes without losing terminal scrollback.
- Make status and snapshot listing with --no-wait print once and exit in interactive terminals.
- Show GPU count selection again when reselecting a GPU type in interactive tnr create, while honoring valid command-line presets.
- Keep commands running when an automatic update fails.
- Verify release archive checksums before publishing and validate complete release artifacts during dry runs.
- Make status --no-wait display the same styled table and last-updated timestamp as monitoring, then exit immediately.
- Retry failed updates after one hour per version, refresh stale checksums, and allow immediate retries with tnr update.
- Preserve macOS and Linux checksums when rebuilding Windows assets, and stop publishing if the existing release cannot be verified.

<!-- Internal changes -->
<!-- Align CLI documentation with Thunder Compute's low-cost GPU positioning for developers. -->
<!-- Consolidate CLI release-note rules into cli/AGENTS.md; the root AGENTS.md now points here instead of duplicating them. -->
<!-- Add a reproducible local self-update check using the published CLI, disposable binaries, and simulated release failures. -->

**Full Changelog**: https://github.com/Thunder-Compute/thunder-cli/compare/v2.0.71...v2.0.72

## v2.0.71 - 2026-07-30

### Fixed
- Keep CLI text legible across light, dark, and limited-color terminal themes.
- Keep browser authentication callback pages readable in light themes.
- Allow create presets to use GPU types provided by the current GPU configuration and show unavailable snapshots.
- Keep installer completion screens readable when the system uses a dark theme.
- Preserve the login link and stable layout when the terminal is resized.
- Keep JSON command output machine-readable and free from interactive text.
- Prevent similarly prefixed instance IDs from colliding in SSH configuration.

### Security
- Harden browser login by validating the OAuth callback state.

<!-- Internal changes -->
<!-- Adds release governance and automation without changing shipped CLI behavior. -->

**Full Changelog**: https://github.com/Thunder-Compute/thunder-cli/compare/v2.0.70...v2.0.71

<!-- Changie baseline: releases through v2.0.70 predate this changelog. -->
