# Scope and acceptance

The project provides one authenticated LAN browser session to an AURGA KVM. The Go binary embeds the UI and implements the device protocol without loading vendor software.

## Implemented

- Passcode interstitial, video-only page and a small mobile-keyboard button.
- First-device LAN discovery, private-IPv4 query override and device-bound, memory-only authentication sessions.
- TLS, Host/Origin checks, login attempt limits and bounded protocol/browser queues.
- H.264/HEVC WebCodecs playback, keyframe refresh/recovery and experimental PCM16 audio.
- Input enabled by default, with explicit `?view=1` diagnostic mode.
- Physical keyboard reports, atomic software-key taps, mouse buttons/wheel and direct touch-to-mouse mapping.
- Best-effort releases on blur/disconnect, with zero-motion mouse release.

## Excluded

- Public Internet exposure, cloud accounts, multiple simultaneous viewers and automatic input replay.
- Device configuration changes, reset, firmware updates and Wake-on-LAN.
- FEC recovery, legacy TCP/USB streaming and arbitrary-Unicode mobile typing.
- Guaranteed A/V synchronisation or host input release after network loss.

## Verification

| Area | Evidence | Limit |
|---|---|---|
| Native protocol and access controls | Unit, malformed-input, race and fuzz tests | Synthetic peers cannot establish device compatibility |
| Full browser bridge | Chromium/WebKit HTTPS → Go → UDP tests | Test peer receives HID; host effects are unverified |
| Touch and keyboard focus | Native browser touch tap/focus; Chromium gesture drag | WebKit drag/wheel cases use DOM events; physical iPad acceptance is unverified |
| Authentication and video | Read-only live firmware 5.0.0.0 checks; 1920×1080 H.264 | Initial smoke checks, not sustained-performance testing |
| Real HID transport | Neutral all-keys-released report ACK | No typed character or click acceptance |
| Audio | Synthetic PCM/configuration/sequence tests and browser scheduling | Audible live output is unverified |

The source release is experimental. Live input tests require an agreed harmless action on an attended host. Never save credentials or real screen contents in test artifacts.
