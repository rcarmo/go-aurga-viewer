# Security boundaries

This is experimental LAN software. It has not received an independent security audit or physical-input acceptance across supported browsers.

## Deployment

- Bind to a specific trusted LAN IP and use HTTPS. Loopback is the default; wildcard binding and LAN plaintext HTTP are rejected.
- Do not forward the service to the public Internet. Use a VPN for remote access.
- Treat a device passcode as permission to control the attached host: input is enabled by default after login. `?view=1` is a diagnostic convenience, not a separate viewer-only authorisation role.
- TLS keys persist in the configured state directory. Authentication tokens and passcodes stay in bounded process memory; restarting invalidates sessions. Protect the service account, process environment and state directory.
- The vendor LAN protocol uses reconstructed authentication/encryption and is not a substitute for a trusted network. Browser-to-bridge traffic uses TLS; bridge-to-device traffic uses the vendor protocol.
- Browser Host/Origin checks and device-bound cookies protect the UI, but authenticated users may select any private IPv4 target. Do not deploy it as a boundary between mutually untrusted networks or users.
- Host input release after network/process failure is best-effort. Concurrent control by another client is untested.

## Reports and testing

Report suspected vulnerabilities privately to the maintainer when a private channel is available. Do not post device passcodes, auth cookies, TLS keys, screen recordings, personal device identifiers or private-network diagnostics in public issues.

The normal test targets use synthetic loopback peers. Live tests require explicit endpoints and credentials. Do not type, click, reset, update or reconfigure real equipment without agreement. Use read-only checks first and do not save real screen contents.
