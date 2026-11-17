# Reconstructed LAN protocol

Evidence: AURGA Android APK 2.3.1.45, SHA-256 `eb84286261de6decc27a1296f82a22ee0c5e5c4dde60d7cbf84f7bb86d50145d`. Its unstripped x86-64 `libcore.so` contains DWARF layouts and named native functions. The Linux 2.1.0.3 client uses the same discovered stream port. Native source/library files are analysis inputs only.

## Provenance

| Implementation | Evidence |
|---|---|
| Discovery text/port | `ScanThread`, live device response |
| Header fields/offsets | DWARF `StreamPacketV2` |
| Header magic/version/type | `ASClientStartSession`, `RecvThread` |
| UDP port 36459 | `ASClientHandleScanning`, `ASClientTestAddresses`, Linux equivalent |
| AES CTR passcode transforms | `as_get_data_by_key1`, `as_get_data_by_key4` |
| Configuration command | Java `QSCtl.getDeviceInfo`, `SocketMessageSender` |
| Handshake response parsing | DWARF `StreamPacketHandshakeResponse`, `RecvThread` |
| Command serialisation/retries | `SendThread`, `ASClientHandleCommandAck` |
| Video 1200-byte fragment offsets | `ASPacketPut2` |
| Per-32-fragment ACK bitmap | `ASPacketFillVideoAck` |
| MD5 frame trailer flag | `ASPacketPut2` |
| Nine-byte keyboard/absolute mouse | Java `PhysicalInputHandler`, `ByteUtils` |
| HID envelope and UDP sequence | `ASClientEnqueueInput`, `SendThread` |

All numeric fields in the packet header are little-endian. A 16-byte header contains magic `0x41564852`, version byte, type byte, total uint16, fragment index uint16, sequence uint32 and payload length uint16. Type bit 7 inverts the payload bytes. Packet types: audio 1, video/ACK 2, input/ACK 3, handshake 4, start 5, keepalive 6, management 8, video status/configuration 20, audio status 21, FEC 22 and connectivity probe 30.

The passcode handshake uses a 16-byte random challenge, a zero-padded 16-byte passcode encrypted using AES-128 CTR and a transformed challenge as key. The native IV is `a2b400aea391e64003f0d4eea29d765b`. This is vendor protocol compatibility, not a security design recommended for new systems. Browser-to-bridge traffic uses TLS.

The configuration API encrypts the variable-length passcode using a different challenge transform. A read-only `get device_info` request authenticated successfully on the real device; secret-bearing returned fields are never exposed by the web API or probe logs.

The session uses queued management commands and input acknowledgements with bounded retries. Initial configuration disables audio and input, enables video and requests refresh. Browser control is opt-in. Release delivery after a broken network is best-effort; the implementation cannot guarantee remote input state after loss of communication.

## Real-device observations, 17 November 2026

- Device `192.168.1.94` reports `AURGA-4B57DC`, firmware `4.0.0.0`, protected.
- UDP discovery and TCP 32465 read-only authentication succeed.
- Both a connectivity probe and passcode handshake sent to UDP 36459 receive ICMP port-unreachable.
- TCP 6851, which was open during the earlier investigation, is now closed. The cause is unknown. No reboot or configuration mutation was requested.
- Later the device began reporting firmware `5.0.0.0`; TCP 6851/10000 and the UDP stream service responded again. No firmware update or reboot was initiated by this implementation.
- Configuration and stream authentication then succeeded with the owner's passcode. The device's handshake reports HDMI state `0` and zero codec configuration bytes. No live video buffers have been received and no live HID input was sent.
- The deployed HTTPS UI was tested in Chromium against this real device: authentication passed, read-only remained enabled, the passcode field cleared and normal disconnect passed. It showed `Connected — no HDMI signal`.
- A separate full synthetic test passed through a real Go HTTPS/WebSocket server and loopback UDP peer: fragmented H.264 decoded to the browser canvas and keyboard/mouse/release reports reached the synthetic peer.

Native function and structure evidence is retained locally under `/workspace/notes/projects/aurga/evidence/`. The initial report is `/workspace/notes/projects/aurga/README.md`.
