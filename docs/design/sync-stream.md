# Sync stream

Status: implemented in the gateway.

## Goal

The app keeps a local copy of the route data tables and stays up to date by pulling only what changed since its last sync. The protocol follows Immich's mobile sync protocol (`POST /sync/stream`, server `sync.service.ts`), with two changes:

1. The server stores no checkpoints. The client stores its own acks and sends them with every request.
2. Every payload is versioned. A newer app can ask for `RoutesV2` while older apps keep getting `RoutesV1`.

## Assumptions

- All synced data is public. There are no users, sessions, or per-client visibility rules, so every client with the same acks gets the same stream.
- The synced tables are the ones in `model.SyncTables` ([sync.go](../../internal/db/model/sync.go)). Each has an `updateId` (uuidv7, bumped by trigger on change) and an audit table of tombstones with a uuidv7 `id`.
- Tombstones are pruned after 31 days ([retention.go](../../internal/db/sync/retention.go)).

## Differences from Immich

| Immich                                                       | Here                                                                 | Why                                                 |
| ------------------------------------------------------------ | -------------------------------------------------------------------- | --------------------------------------------------- |
| `session_sync_checkpoint` table, `GET/POST/DELETE /sync/ack` | No table, no ack endpoints. Acks go in the stream request body.      | No server-side sync state.                          |
| `reset: true` in the request marks the session for reset     | Dropped. The client resets by wiping local data and sending no acks. | Nothing on the server to mark.                      |
| `SyncAckV1` for backfills of newly shared data               | Dropped.                                                             | No per-user visibility, so nothing gets backfilled. |
| Entity DTOs listed as extra models                           | Stream line is also published as a `oneOf` keyed on `type`.          | Generated clients can narrow on `type`.             |

Everything else matches Immich: the line format, ack format, request and entity types, order, the `nowId` upper bound, `SyncResetV1`, and `SyncCompleteV1`.

## Endpoint

`POST /api/sync/stream`

Request:

```json
{
  "types": ["RoutesV1", "StopsV1"],
  "acks": ["RouteV1|0199...", "RouteDeleteV1|0199...", "SyncCompleteV1|0199..."]
}
```

Response: `200`, `Content-Type: application/jsonlines+json`, one JSON object per line:

```json
{"type":"RouteDeleteV1","ack":"RouteDeleteV1|0199...","data":{"routeId":"0199..."}}
{"type":"RouteV1","ack":"RouteV1|0199...","data":{"id":"0199...","shortName":"01", ...}}
{"type":"SyncCompleteV1","ack":"SyncCompleteV1|0199...","data":{}}
```

`400` if:

- an ack has an unknown entity type or its id is not a uuidv7
- `types` has two versions of the same resource, for example `RoutesV1` and `RoutesV2`

Acks for entity types the request does not use are ignored.

## Types

There are two enums, same as Immich.

- `SyncRequestType` is what the client asks for, one per resource per version.
- `SyncEntityType` is the `type` of each line. Each request type produces one upsert entity and one delete entity.

| Request type        | Upsert entity      | Delete entity            | Table             |
| ------------------- | ------------------ | ------------------------ | ----------------- |
| `RoutesV1`          | `RouteV1`          | `RouteDeleteV1`          | `route`           |
| `DirectionsV1`      | `DirectionV1`      | `DirectionDeleteV1`      | `direction`       |
| `StopsV1`           | `StopV1`           | `StopDeleteV1`           | `stop`            |
| `DirectionStopsV1`  | `DirectionStopV1`  | `DirectionStopDeleteV1`  | `direction_stop`  |
| `AlertsV1`          | `AlertV1`          | `AlertDeleteV1`          | `alert`           |
| `AlertDirectionsV1` | `AlertDirectionV1` | `AlertDirectionDeleteV1` | `alert_direction` |
| `StopSchedulesV1`   | `StopScheduleV1`   | `StopScheduleDeleteV1`   | `stop_schedule`   |

Control entities, which have no table:

| Entity           | Data | Meaning                                                                        |
| ---------------- | ---- | ------------------------------------------------------------------------------ |
| `SyncResetV1`    | `{}` | The client's acks can no longer be trusted. It is the only line in the stream. |
| `SyncCompleteV1` | `{}` | The stream finished. Its ack carries `nowId`.                                  |

Each entity type has a payload schema named `Sync<Entity>`: `RouteV1` carries `SyncRouteV1` and `RouteDeleteV1` carries `SyncRouteDeleteV1`. Delete payloads carry the audit key, for example `{"routeId": "..."}`.

## Acks

The format is `<SyncEntityType>|<uuid>`, the same as Immich's `toAck`. The uuid is the row's `updateId` for upserts, the tombstone `id` for deletes, and `nowId` for `SyncCompleteV1`.

The client:

- keeps the latest ack per entity type, keyed by the part before `|`
- writes each ack in the same local transaction as the rows it covers, so data and acks cannot drift after a crash or dropped connection
- sends every stored ack on the next request
- drops the acks for a request type when it stops requesting it

The server parses the acks into a cursor keyed by entity type. It never trusts an ack beyond "rows after this id". A forged or stale ack only affects that one client.

## Stream

1. Parse and validate the request.
2. If the `SyncCompleteV1` ack was minted more than 30 days ago, send `SyncResetV1` and end the stream. Tombstones older than 31 days may be gone, so the client could be missing deletes. `Cursor.Stale` already does the age check.
3. Mint `nowId = uuidv7(now() - 1ms)` in Postgres. Every query below is bounded by `< nowId`, so rows written during the stream wait for the next sync.
4. For each requested type in the server's fixed order:
   1. deletes: tombstones with `id > ack`, `id < nowId`, ordered by `id`
   2. upserts: rows with `updateId > ack`, `updateId < nowId`, ordered by `updateId`
5. Send `SyncCompleteV1` with `nowId` and end the stream.

With no ack for an entity type, its query starts from the beginning. A client with no acks at all gets a full sync. It gets no deletes that matter, because it has nothing to delete.

The fixed order follows foreign keys, so parents arrive before children: routes, directions, stops, direction stops, alerts, alert directions, stop schedules. All versions of a resource sit at the same position. The order of `types` in the request does not matter.

Write lines with backpressure and stop when the client disconnects, like Immich's `send`.

### Reset

When the client gets `SyncResetV1`, it wipes every synced table and every stored ack in one local transaction, then requests again with no acks. The server does not have to remember anything for this.

## Versioning

- A payload schema never changes once shipped. Adding, removing, or retyping a field means a new entity version (`RouteV2`) and a new request version (`RoutesV2`).
- A new request version reuses the delete entity unless the delete payload changes too. `RoutesV2` produces `RouteV2` and `RouteDeleteV1`.
- Acks are keyed by entity type, so the first `RoutesV2` request has no `RouteV2` ack and gets every route in the new shape. This matches how Immich moved from `AuthUsersV1` to `AuthUsersV2`. The existing `RouteDeleteV1` ack still applies, so the client does not refetch deletes.
- Payloads are explicit Go structs mapped from the models, not generated from columns. The `sync:"nosync"` tag still decides which column changes bump `updateId`. A column that only `RouteV2` uses must be synced, so V1 clients also get those rows again when it changes. Re-applying an identical row is harmless.
- An old request type keeps being served until no supported app version sends it. After that it is marked deprecated and its handler becomes a no-op, like Immich's `syncAssetsV1`, so old apps get an empty section instead of a `400`.

## OpenAPI

- `SyncRequestType` and `SyncEntityType` are named enum components, through the existing `EnumValues` pass.
- Each `Sync<Entity>` payload is a component.
- `SyncStreamLine` is a `oneOf` of one object per entity type, `{type: const, ack: string, data: $ref}`, with `discriminator.propertyName: type`. fuego cannot express this, so it is built in the post-processing pass from the entity → payload map below.
- The stream response is `application/jsonlines+json` with schema `SyncStreamLine`. The schema describes one line.

## Code changes

- [dtos/sync.go](../../apps/gateway/dtos/sync.go): the `SyncRequestType` and `SyncEntityType` enums, `SyncRequest`, and `SyncStreamLine`.
- [dtos/sync_payloads.go](../../apps/gateway/dtos/sync_payloads.go): the `Sync<Entity>` payloads, their model mappers, and `SyncPayloads`, the entity → payload map the OpenAPI union is built from.
- [services/sync.go](../../apps/gateway/services/sync.go): `SyncStreams`, the ordered registry of request types. Ack parsing, planning, and streaming.
- [repositories/sync.go](../../apps/gateway/repositories/sync.go): `NowID`, plus `Upserts` and `Deletes` iterators bounded by ack and `nowId`.
- [controllers/sync.go](../../apps/gateway/controllers/sync.go): writes JSON lines straight to the response. The status line waits for the first line, so an error before any data still gets a proper error response.
- [controllers/sync_openapi.go](../../apps/gateway/controllers/sync_openapi.go): builds the `SyncStreamLine` union.

### Adding a version

1. Add `RoutesV2` to `SyncRequestTypes` and `RouteV2` to `SyncEntityTypes`.
2. Add `SyncRouteV2` and `NewSyncRouteV2`, and register the payload in `SyncPayloads`.
3. Add `NewSyncStream(RoutesV2, RouteV2, RouteDeleteV1, NewSyncRouteV2, NewSyncRouteDeleteV1)` next to `RoutesV1` in `SyncStreams`.

The unit specs fail if any of these is missing.

## Open questions

1. Rows committed out of `updateId` order can be skipped. The bump trigger mints `updateId` with `clock_timestamp()` inside the writing transaction. If a requester transaction mints an id, then another transaction commits a later id, and a client syncs between the two commits, that client acks past the first id and never sees that row. Immich has the same gap. The requester does not open explicit transactions today, so each window lasts one bulk upsert statement. Pulling `nowId` back by more than the slowest upsert, for example a few seconds, closes the gap.
2. Is 30 days without opening the app the right cutoff for a full reset?
