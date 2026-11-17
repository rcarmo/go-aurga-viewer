# aurga-web

A Go bridge for using an AURGA Viewer KVM from a web browser. It streams video and audio and forwards keyboard, mouse and touch input over the LAN. A single binary includes the web interface; no cloud account or official application is required.

I wanted my own browser front-end because I found the official application flaky and temperamental. See [my original review of the AURGA Viewer](https://taoofmac.com/space/blog/2024/09/29/1900).

This is an independent project, not an official AURGA product.

## Build and run

Requires Go 1.25 or newer, Make and a browser with WebCodecs.

```sh
make run
```

This builds and starts the server. Open **https://127.0.0.1:9443/**, accept the self-signed certificate and enter the device passcode. Use `make` to build without starting it.

To connect from another LAN device, listen on the server's LAN address:

```sh
make run ARGS="-listen 192.168.10.10:9443"
```

Replace the example address with your server's address, then open that address in your browser. The default listener accepts local connections only; wildcard listeners are rejected.

The bridge selects the first device discovered on the LAN. To choose another device, add `?ip=192.168.10.50` to the viewer URL or set a startup default with `-device`. URL overrides must be private IPv4 addresses.

TLS certificates persist in the user configuration directory under `aurga-web`, or in `-state-dir`. To use your own certificate, place `cert.pem` and `key.pem` in that directory.

## Controls

Keyboard and mouse control are enabled by default. **Add `?view=1` to the URL for a read-only session.**

- Click the video to focus keyboard input. The keyboard button opens a text input on mobile devices. Mobile text entry supports ASCII, Enter and Backspace with a US host keyboard layout; arbitrary Unicode is unsupported.
- Use the mouse buttons and wheel normally. Where supported, clicking captures the mouse; press Esc to release it. Otherwise, pausing at a video edge pans the remote cursor.
- Touch works as a trackpad: swipe to move the cursor, tap to click, or hold for half a second then move to drag. `?mouse=absolute` enables direct-coordinate mouse and touch input on compatible hosts.
- Tap the video or keyboard button if audio does not start, including after returning to Safari.
- The **i** button shows connection and input diagnostics.

Only one viewer can connect at a time. Concurrent use with the official application is untested. If a connection failure leaves remote input stuck, use the official application to release it.

## Compatibility

The project is experimental. Video, audio and input work on my setup with firmware 5.0.0.0, including 1920×1080 H.264 video and iPad use. Compatibility with other firmware and devices has not been verified.

Current Chromium browsers are the primary target. H.264 and HEVC availability depends on the browser and operating system. Browser autoplay rules and operating-system interruptions can pause audio; audio/video synchronisation is not guaranteed.

Device configuration changes, firmware updates and Wake-on-LAN are not supported.

## Security

**Keep the service on a trusted LAN. Use a VPN for remote access; do not expose it to the public Internet.** HTTPS is required for LAN connections.

Login creates a device-bound session cookie valid for 24 hours. Passcodes and sessions are held in server memory; restarting the bridge requires logging in again. Passcodes are not stored in cookies, browser storage or logs.

The bridge normally checks the passcode with the device. Set `AURGA_PASSCODE` to check it locally instead. Supply secrets through your service or secret manager, not command-line arguments or committed files.

See [SECURITY.md](SECURITY.md) for deployment restrictions and reporting vulnerabilities.

## Linux service

Use [docs/aurga-web.service](docs/aurga-web.service) as a systemd example. Create the `aurga` account, install the binary at `/usr/local/bin/aurga-web`, install the unit and configure `AURGA_LISTEN` in `/etc/aurga-web.env` before enabling LAN access. The service stores TLS files in `/var/lib/aurga-web` and defaults to listening on loopback.

## Development

```sh
make verify-modules check race
make browser-e2e
AURGA_BROWSER=webkit make browser-e2e
```

Browser tests require Bun, ffmpeg, Playwright and matching Chromium/WebKit builds. Run `bun install` for the pinned development dependency, then install the browsers with Playwright. Set `PLAYWRIGHT_BROWSERS_PATH` to use an existing browser installation. These tests use simulated devices and do not contact a real KVM.

See the [protocol notes](docs/protocol.md) and [project scope](docs/scope.md) for implementation details.

To create a source archive and checksum from a clean, committed tree:

```sh
make dist DIST_DIR="$HOME/releases"
```

## Licence

[MIT](LICENSE), copyright 2026 Rui Carmo. Distributions must retain the applicable licence notices; see [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
