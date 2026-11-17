# First-version scope

Rui requested a simple browser replacement for frequent access to the LAN AURGA KVM, preferably in Go. The first build targets one user, one configured device and a current desktop Chromium browser.

## Refinement decisions

1. Problem: vendor app makes frequent KVM access inconvenient.
2. User: Rui, on the existing trusted LAN.
3. Success: opening one URL gives video and explicitly enabled keyboard/mouse control.
4. Minimum version: HTTPS UI, passcode authentication, HDMI video, keyboard and absolute mouse.
5. Exclusions: audio, mobile polish, cloud accounts, firmware/configuration changes and WAN exposure.
6. Surfaces: Go protocol package, HTTP/WebSocket bridge, embedded static UI, tests, service example.
7. Platform: pure Go server, browser WebCodecs; no vendor runtime or Node/Bun runtime.
8. Failures: show the device/protocol error, stop the session, disable input, bound retry/timeouts.
9. Persistence: TLS certificate/key only; no passcodes, screen recordings or browser credential storage.
10. UI pattern: one connection form, screen, read-only default, explicit control toggle.
11. First slice: discovery, protocol fixtures and authentication before input.
12. Avoid: configuration mutation, arbitrary destination addresses, automatic reconnect/input replay.
13. Project: `aurga-web`, binary `aurga-web`; Go source and plain HTML/CSS/JS.
14. Inputs: fixed startup device IP, session passcode and authorised HID events. Output: video and status.
15. Existing behaviour: preserve device settings and vendor-app access; concurrent use is untested.
16. Performance: prefer direct H.264 browser decode; bounded frame, message and input queues.
17. Security: HTTPS, Host/Origin checks, local attempt limit, one session and explicit input enable.
18. Deliverables: source archive and standalone Go binary; exclude secrets and vendor binaries.
19. Tests: protocol vectors, malformed datagrams, simulated device, race detector, real browser decode and real-device acceptance.
20. Done: sustained live video, usable keyboard/mouse, release on disconnect, and a verified restart procedure.

These choices follow the existing request where it answers the baseline questions. Browser/OS coverage and performance targets can be expanded after the first real-device session works.

## Current acceptance

Native tests, race detection and a full synthetic UDP → Go → WebSocket → browser decoding/control test pass. Discovery, read-only metadata authentication and stream authentication pass against the real device, now reporting firmware 5.0.0.0. The deployed HTTPS UI has passed real-device authentication and read-only/disconnect checks. Live video and HID acceptance have not passed because the device reports no HDMI signal. The service is an experimental build until sustained video and control are verified.
