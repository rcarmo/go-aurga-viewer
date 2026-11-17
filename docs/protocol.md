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

The session uses queued management commands and input acknowledgements with bounded retries. Session configuration activates streaming, applies the requested audio/input permissions, enables video and requests refresh. The web viewer enables input by default after operation-2 acknowledgement; diagnostic `?view=1` sessions disable it. Individual input ACKs pace browser reports. Physical keyboards use report 1; virtual text uses the vendor's atomic report 10 (9 bytes: ID, modifiers, reserved, key slots), matching `Java_com_qstec_shared_stream_StreamClient_sendCharInput`, `ascii2usbcode` and `InvisibleKeyboardView.sendExtensionKey`. Software taps are retained across focus transitions. Native Touch Events handle iPad touch; mouse/pen use Pointer Events. Blur/disconnect release uses a zero-motion relative mouse report to avoid moving the pointer to the origin. Release delivery after a broken network is best-effort; the implementation cannot guarantee remote input state after loss of communication.

## Audio and browser transport

Audio type 1 contains PCM signed 16-bit little-endian samples. The native `RecvThread` passes each newer packet directly to `ASAudioPlayerFeed`; no video fragment assembly or ACK is used. Handshake offsets 2/3/4 contain audio state, channel count and uint16 sample rate. Type 21 supplies uint32 channels/rate at offsets 0/4 and receives an empty ACK. Operation 1 enables audio. Channel/rate/payload bounds and sequence checks reject malformed or stale PCM.

WebSocket binary messages use a one-byte tag: 0 for Annex-B video, 1 for PCM. JSON audio events supply channels/sampleRate. Web Audio buffers perform PCM conversion and sample-rate conversion; queued playback is bounded to 250 ms. Synthetic audio configuration, duplicate rejection and browser scheduling pass. The tested live source has not advertised audio, so audible output is unverified.

The browser logs in over POST /api/auth. A random device-bound token goes into a 24-hour HttpOnly Secure SameSite=Strict cookie; passcodes and sessions stay in bounded server memory. The default target is the first private-IPv4 LAN discovery response; ?ip=private-IPv4 overrides it. Public addresses, hostnames, loopback and IPv6 query overrides are rejected.

## Compatibility evidence

Firmware 5.0.0.0 passed LAN discovery, read-only configuration authentication, stream authentication and initial 1920×1080 H.264 rendering in Chromium and WebKit. A no-HDMI handshake reports state 0 and no codec configuration; an active source reports state 1 with codec configuration. These observations do not establish sustained performance or compatibility with every firmware revision.

An all-keys-released report received a real-device input ACK. Synthetic tests verify keyboard, software-key taps, mouse and touch report delivery through the complete bridge. Physical iPad input and host-side effects need acceptance; audible output has not been verified. The project does not distribute host screen recordings, device identities, research binaries or decompiled listings.
