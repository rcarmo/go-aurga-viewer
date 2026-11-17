# aurga-web

A standalone Go bridge that opens an AURGA KVM's HDMI stream in a browser. The binary embeds the UI and connects directly over the LAN. It needs no cloud account, vendor runtime, Node or Bun at runtime.

I wanted my own browser front-end because I found the official application flaky and temperamental. For background, see [my original review of the AURGA Viewer](https://taoofmac.com/space/blog/2024/09/29/1900).

**Experimental:** live 1920×1080 H.264 video, discovery and authentication have been verified with firmware 5.0.0.0. Keyboard, mouse, touch and software-keyboard paths pass synthetic end-to-end tests in Chromium and WebKit. Video, audio and input have also been confirmed working on my setup, including iPad use. Compatibility and sustained streaming performance on other setups remain unverified.

This is an independent interoperability project, not an official AURGA product.

## Build and run

Requires Go 1.25 or newer, Make and a browser with WebCodecs.

```sh
make deps check
BIN=$(make -s print-binary)
"$BIN"
```

Open **https://127.0.0.1:9443/**, accept the self-signed certificate and enter the device passcode. The default listener is loopback-only. To reach it from another LAN device, replace the example address with the server's own LAN IP:

```sh
"$BIN" -listen 192.168.10.10:9443
```

The first LAN discovery reply selects the device. An explicit `?ip=192.168.10.50` URL query overrides it; `-device` sets an optional startup default. Query overrides must be private IPv4 addresses. Wildcard listeners, non-loopback plaintext HTTP, public targets, loopback targets, hostnames and IPv6 query overrides are rejected.

TLS certificates persist in the user configuration directory under `aurga-web`, or in `-state-dir`. The private key is mode 0600. You can supply a trusted certificate as `cert.pem` and `key.pem` in that directory.

## Viewer

- A passcode interstitial opens a video page with small keyboard and input-diagnostics buttons.
- The **i** button shows browser event/report counters, bridge sends/ACKs and audio format/packet/playback counters. Diagnostic logs contain counts and protocol operation numbers only, not keys, coordinates, PCM samples or passcodes.
- Keyboard and mouse are enabled by default after the device acknowledges negotiation. **Use `?view=1` for a read-only session.**
- Mouse buttons and wheel use relative HID reports by default for host compatibility. Touch acts as a trackpad: swipe to move the cursor, tap to click at its current position, or hold for half a second then move to drag. Native Touch Events handle iPad touch; Pointer Events handle mouse/pen. `?mouse=absolute` restores direct-coordinate mouse/touch input on hosts that support it. Clicking captures the mouse where Pointer Lock is supported; Esc releases it. Other browsers use bounded edge panning when the mouse dwells at the video boundary. Large relative movements are split across reports rather than clipped.
- The keyboard button opens a mobile text input from its touch gesture. Text uses the vendor's atomic software-keyboard report. ASCII, Enter and Backspace are supported with a US host keyboard layout; arbitrary Unicode is unsupported.
- Reports are paced by device acknowledgements and bounded queues. Audio/video have separate lossy queues, so media pressure cannot fill the control/ACK queue. Stale video is dropped and a keyframe requested without reconnecting; genuine transport/input timeouts still stop the session. Blur and normal disconnect send release reports. A broken network can prevent remote release; use the official app if input becomes stuck.
- One viewer is admitted at a time. Concurrent use with the official app is untested.

Current Chromium browsers are the primary target. WebKit tests cover video, native touch tap and keyboard focus; their drag/wheel cases dispatch DOM events and do not establish physical iPad acceptance. H.264/HEVC availability depends on the browser and operating system.

Audio is experimental PCM16 playback through Web Audio. Safari interruptions retain the stream format: returning to the page attempts resume, and the next touch/click/key gesture rebuilds a context that remains interrupted. A playback AudioSession is requested where supported; operating-system interruptions may still pause output. Autoplay rules require a tap after cookie auto-login; the keyboard button also enables audio. The tested live source did not advertise an audio format. A/V synchronisation is not guaranteed.

## Authentication and security

Login issues a random 256-bit, device-bound cookie for 24 hours. It is HttpOnly, Secure and SameSite=Strict over HTTPS. The server stores passcodes and token mappings in bounded memory only; restarting invalidates them. Passcodes are not written to cookies, browser storage, logs or project files. Loopback HTTP development uses a non-Secure cookie.

The bridge normally checks the passcode through the read-only device API. Optional `AURGA_PASSCODE` checks browser credentials locally instead. Supply secrets through your service or secret manager, not command-line arguments or committed files.

Passcode attempts are limited to five per minute per source IP. Host/Origin checks, message limits, CSP and HTTPS protect the browser interface. **Do not expose the service to the public Internet.** Use a VPN for remote access and review [SECURITY.md](SECURITY.md).

## Tests

```sh
make verify-modules check race
make browser-e2e
AURGA_BROWSER=webkit make browser-e2e
```

Browser tests additionally need Bun, ffmpeg, Playwright and matching Chromium/WebKit builds. Use `bun install` for the pinned development dependency, then install the browsers with Playwright. Make automatically uses shared workspace browsers when available; `PLAYWRIGHT_BROWSERS_PATH` can select another installation.

`make browser-test` intercepts the WebSocket with a synthetic peer and verifies decoding, software-key taps and PCM scheduling. `make browser-e2e` runs a real Go HTTPS/WebSocket bridge and loopback UDP peer, testing fragmented video, input reports, touch handling, focus transitions, cookie reload and lost-keyframe recovery. These targets do not contact a real KVM or record a real screen.

Explicit read-only diagnostics:

```sh
# Replace the example with the device's private IPv4 address.
make probe DEVICE=192.168.10.50
# Also attempt a six-second video session, without saving frames:
make probe DEVICE=192.168.10.50 PROBE_FLAGS=-stream
```

Inject `AURGA_PASSCODE` when authentication is required. `-trace` logs packet headers only. Live browser smoke tests require explicit `AURGA_TEST_URL` and `AURGA_PASSCODE`; they add `?view=1` and save no screen contents. The optional neutral keyboard ACK test requires `AURGA_LIVE_INPUT_ACK=1` and `AURGA_LIVE_DEVICE`; it sends an all-keys-released report and must be agreed before use. Do not run live input tests against an unattended host.

## Linux service

[docs/aurga-web.service](docs/aurga-web.service) is a systemd example using `/usr/local/bin/aurga-web`, a dedicated `aurga` account and `StateDirectory=aurga-web` for TLS files. Create the account, install the built binary and unit, and configure `AURGA_LISTEN` in `/etc/aurga-web.env` before enabling LAN access. Its default is loopback-only. The sample does not store a device passcode.

## Source releases

Commit reviewed changes, then create a history-free source archive and checksum in a durable directory:

```sh
make dist DIST_DIR="$HOME/releases"
```

The target packages the committed tree and refuses a dirty working tree. It does not publish, tag or rewrite Git history. The archive includes source, tests and notices, not vendor binaries, local credentials, screen captures, build caches or Git history. Binary distributors must retain this project's and dependencies' licence notices.

## Protocol and limits

The implementation reconstructs protocol behaviour from AURGA client 2.3.1.45 for interoperability. No vendor binaries or decompiled listings are distributed. See [protocol notes](docs/protocol.md), [scope](docs/scope.md) and [third-party notices](THIRD_PARTY_NOTICES.md).

Discovery uses UDP 32469, configuration authentication TCP 32465 and streaming UDP 36459. Implemented paths include protocol-v3 authentication, queued commands, bounded retries, fragment assembly/ACK bitmaps, optional frame MD5 checks, Annex-B video and HID reports. FEC recovery, legacy TCP/USB transports, device configuration changes, firmware updates and Wake-on-LAN are not implemented.

## Build paths

Make snapshots inherited `TMPDIR` before selecting a project-scoped temporary root. `WORKSPACE_TMP_BASE` overrides the base. In CI, the fallback is `RUNNER_TEMP`, inherited `TMPDIR`, then `/tmp`; locally it prefers writable `/workspace/tmp`, then inherited `TMPDIR` or `/tmp`. Unset, empty, `0` and `false` CI flags mean local mode. Every root ends in `aurga-web`.

Subdirectories are `cache/go-build` (`GOCACHE`), `cache/go-mod` (`GOMODCACHE`), `cache/bun` (`BUN_INSTALL_CACHE_DIR`), `build` (`GOTMPDIR` and binaries), `runs` (`TMPDIR`), `tests` and `logs`. TLS state and release archives belong outside disposable storage. Direct Go commands can reuse this environment or use normal Go defaults.

## Licence

[MIT](LICENSE), copyright 2026 Rui Carmo. Dependencies retain their own licences; see [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
