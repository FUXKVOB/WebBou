# WebBou Wire Specification

## Version

- status: production
- wire version: `v1`
- transport: `TCP + TLS`
- port: `8443`

## Handshake

```text
Client                          Server
  |                               |
  |--- HELLO -------------------->|
  |<-- HELLO_ACK -----------------|
  |                               |
  |==== DATA / PING / PONG ======>|
```

`HELLO` is required before application data.

## Frame Header

The frame header is always 16 bytes.

| Offset | Size | Field |
| --- | --- | --- |
| 0 | 1 | magic = `0xB0` |
| 1 | 1 | version = `0x01` |
| 2 | 1 | frame type |
| 3 | 1 | flags |
| 4 | 4 | stream id, big-endian |
| 8 | 4 | payload length, big-endian |
| 12 | 4 | CRC32 over `magic + version + type + flags + stream_id + payload_length + payload` |

## Supported Frame Types

| Value | Name | Supported |
| --- | --- | --- |
| `0x01` | `DATA` | yes |
| `0x02` | `PING` | yes |
| `0x03` | `PONG` | yes |
| `0x04` | `STREAM_OPEN` | yes |
| `0x05` | `STREAM_CLOSE` | yes |
| `0x10` | `HELLO` | yes |
| `0x11` | `HELLO_ACK` | yes |
| `0x06` | `ACK` | planned |
| `0x07` | `RESET` | planned |
| `0x08` | `SETTINGS` | planned |
| `0x12` | `HELLO_DONE` | planned |
| `0x20` | `MULTI_PATH` | planned |
| `0x21` | `PATH_CLOSE` | planned |
| `0x30` | `FLOW_CONTROL` | planned |
| `0x31` | `MAX_DATA` | planned |
| `0x32` | `BLOCKED` | planned |
| `0x33` | `ACK2` | planned |

## Supported Flags

| Value | Name | Supported |
| --- | --- | --- |
| `0x01` | `COMPRESSED` | yes |
| `0x02` | `ENCRYPTED` | yes |
| `0x04` | `RELIABLE` | yes |
| `0x08` | `PRIORITY_HIGH` | parsed only |
| `0x10` | `FRAGMENTED` | reserved |
| `0x20` | `FINAL` | reserved |
| `0x40` | `ZERO_RTT` | reserved |
| `0x80` | `MULTI_PATH` | reserved |

## Golden Fixture

The shared golden fixture for serialization lives at:

- `protocol/testdata/data_frame_v1.hex`

It represents:

- type: `DATA`
- flags: `0`
- stream id: `7`
- payload: `ping`
