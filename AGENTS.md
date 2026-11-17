# aurga-web development

The Go binary embeds a LAN browser viewer. The default target is the first private-IPv4 discovery reply; `?ip=` overrides it and `-device` sets an optional default. Cookies are device-bound and backed by bounded server memory.

## Safety

- Never change device settings, reset/update firmware or send unsolicited input.
- Input is enabled by default in the UI. Diagnostic live checks must use `?view=1` or `Session.InitialInput=false`.
- Live key/click actions require explicit agreement. The opt-in neutral keyboard ACK test sends an all-keys-released report; it is still a live HID action.
- Never commit passcodes, tokens, TLS keys, screen captures, vendor binaries or decompiled listings.
- Source compatibility comes from protocol research; keep function references in `docs/protocol.md`. Do not describe this work as a clean-room implementation.

## Verification

`make` builds the binary; `make run` builds and starts it. Pass runtime flags with `make run ARGS="..."`. Existing explicit build/test targets and `print-binary` remain available.

Use `make check`, `make verify-modules` and `make race`. UI/input changes also require `make browser-e2e` and `AURGA_BROWSER=webkit make browser-e2e`. Browser E2E uses a loopback synthetic peer, not a live KVM. WebKit's native tap/focus is tested; DOM drag/wheel tests do not establish physical iPad acceptance. Software-keyboard taps use vendor report 10; physical held-state uses report 1. Distinguish implemented, synthetic-tested and live-accepted behaviour.

Ordinary tests do not collect profiles. Measure representative workloads during performance work, retain concise findings and remove owned raw profiling captures after analysis.

## Build and temporary paths

`WORKSPACE_TMP_BASE` overrides the base. Otherwise CI (except unset, empty, `0` or `false`) uses `RUNNER_TEMP`, inherited `TMPDIR`, then `/tmp`. Local builds prefer writable `/workspace/tmp`, then inherited `TMPDIR` or `/tmp`. Always append `aurga-web`; no worktree suffix is currently defined. Make snapshots inherited `TMPDIR` before exporting scoped values.

Layout: `runs` (`TMPDIR`), `cache/go-build` (`GOCACHE`), `cache/go-mod` (`GOMODCACHE`), `cache/bun` (`BUN_INSTALL_CACHE_DIR`), `build` (`GOTMPDIR` and binaries), `tests`, and `logs`. No project-local caches. Do not clean directories used by another process. Shared installed browsers may use `/workspace/bin/pw-browsers`; `PLAYWRIGHT_BROWSERS_PATH` can override this.

TLS state and curated release archives belong outside disposable paths. `make dist DIST_DIR=/durable/path` packages a clean committed tree without Git history. Retain `LICENSE` and `THIRD_PARTY_NOTICES.md` in distributions; do not publish local history without a separate review.
