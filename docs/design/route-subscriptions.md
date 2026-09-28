# Route subscriptions for next departure times

Status: proposed, not implemented.

## Goal

The requester currently fetches next departure times for every stop on every active route every 10 seconds ([live_data.go](../../apps/requester/jobs/live_data.go)). It should only fetch departures for routes that have at least one live WebSocket subscriber. When a route gets its first subscriber, the requester should fetch that route right away instead of waiting for the next tick.

## Assumptions

- There is exactly one requester instance. No leader election or advisory locks.
- The WebSocket server is a separate process and may run more than one instance.
- Clients subscribe per route. There is no per-stop subscription.
- Postgres is the only shared infrastructure. No Redis, no HTTP calls between the servers.

## Overview

Postgres holds two things:

1. `route_subscription`, a lease table. This is the source of truth for which routes have subscribers.
2. `LISTEN/NOTIFY` channels. These are wake-up signals only. Losing one must never cause wrong state, only a delay until the next tick.

```
WS server                         Postgres                          Requester
---------                         --------                          ---------
first local sub on route  ──►  INSERT route_subscription
                               AFTER INSERT trigger
                               pg_notify('route_subscribed')  ──►  LISTEN, debounce,
                                                                   fetch that route
heartbeat every 15s       ──►  UPDATE expiresAt
                                                                   10s tick: fetch all
                                                                   subscribed routes
                               pg_notify('departures_updated') ◄── after SyncDepartures
LISTEN, read departures,  ◄──
push to clients
```

## Schema

Columns follow the existing quoted camelCase convention used by `route`, `stop`, and `direction`.

```sql
CREATE TABLE "route_subscription" (
    "instanceId" uuid        NOT NULL,
    "routeId"    uuid        NOT NULL REFERENCES "route" ("id") ON DELETE CASCADE,
    "expiresAt"  timestamptz NOT NULL,
    PRIMARY KEY ("instanceId", "routeId")
);

CREATE INDEX ON "route_subscription" ("expiresAt");

CREATE FUNCTION notify_route_subscribed() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('route_subscribed', NEW."routeId"::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER route_subscription_notify
    AFTER INSERT ON "route_subscription"
    FOR EACH ROW EXECUTE FUNCTION notify_route_subscribed();
```

Why the trigger fires only on new rows: an upsert that hits `ON CONFLICT DO UPDATE` fires UPDATE triggers, not INSERT triggers. Heartbeats and re-subscribes on an already-leased route produce no notification. Postgres delivers NOTIFY at commit, so the requester never sees a route ID before its row is visible.

Add this as a new migration in [internal/db/migrations](../../internal/db/migrations).

## WebSocket server responsibilities

Each WS process generates an `instanceId` (UUID) at startup.

It keeps an in-memory count of connections per route. The DB sees one row per instance per route, not one per connection, so connection churn does not turn into DB writes.

| Event | DB action |
|---|---|
| Route goes from 0 to 1 local subscribers | `INSERT ... ON CONFLICT ("instanceId", "routeId") DO UPDATE SET "expiresAt" = EXCLUDED."expiresAt"` with `expiresAt = now() + LEASE_TTL` |
| Heartbeat, every `HEARTBEAT_INTERVAL` | `UPDATE "route_subscription" SET "expiresAt" = now() + LEASE_TTL WHERE "instanceId" = $1` |
| Route goes from 1 to 0 local subscribers | Nothing. Stop renewing that route and let the lease expire. |
| Graceful shutdown | `DELETE FROM "route_subscription" WHERE "instanceId" = $1` |

Letting leases expire gives a grace period. A phone that drops and reconnects within `LEASE_TTL` does not cause a new INSERT or a new immediate fetch.

A crashed instance stops heartbeating and its rows expire on their own.

Heartbeats must only renew routes that still have local subscribers. Either run the UPDATE with `AND "routeId" IN (...)`, or delete the idle rows once the grace period passes.

The WS server also runs `LISTEN departures_updated`. On each notification it reads current departures for that route from the DB and pushes them to that route's subscribers.

When a client subscribes to a route that already has a live lease, the WS server sends the departures already in the DB. It does not wait for a notification.

## Requester responsibilities

### Split departures out of the live data job

Vehicles stay in `NewLiveDataJob` on the 10s gocron schedule. Move the next departures part (everything from `GetStopDirectionSourceIDs` through `SyncDepartures`) into its own worker. Do not trigger departures through `scheduler.RunNow(JobLiveData)`, because that reruns the vehicle fetch too.

### Departures worker

One goroutine owns all departure fetches, started and stopped through fx lifecycle hooks. It selects on:

- a 10s ticker, which fetches all routes in `SELECT DISTINCT "routeId" FROM "route_subscription" WHERE "expiresAt" > now()`
- a channel of route IDs fed by the listener, which collects IDs for `SUBSCRIBE_DEBOUNCE` and then fetches only those routes

Both paths run on the same goroutine, so two syncs for the same stop never run at the same time.

If the tick finds no subscribed routes, it skips the upstream calls entirely.

### Listener

A dedicated pgx connection, not taken from the bun pool, runs `LISTEN route_subscribed` and loops on `conn.WaitForNotification(ctx)`. Each payload is a route UUID sent to the worker channel.

When the connection drops, reconnect with backoff. After reconnecting, trigger one full fetch, because notifications sent while it was disconnected are lost.

### Repository changes

- `GetStopDirectionSourceIDs` takes the list of route IDs and adds `WHERE r."id" IN (...)`. Each stop then only gets route/direction pairs for subscribed routes.
- `SyncDepartures` has to be fixed first. Its stale-row delete is scoped by `stopId` only ([route_data.go](../../apps/requester/repositories/route_data.go), the `staleDepartures` query). Once the requester fetches only some routes at a stop, that delete wipes departures for the other routes at the same stop. Scope the delete to the fetched routes, for example `"stopId" IN (...) AND "routeId" IN (...)`.
- `UpdateStopAmenities` only updates stops that were fetched, so it is already safe with partial fetches.
- Add a cleanup step to [db_cleanup.go](../../apps/requester/jobs/db_cleanup.go) that deletes `route_subscription` rows whose `expiresAt` is more than a few minutes old.

### After a sync

After `SyncDepartures` succeeds, call `pg_notify('departures_updated', routeId)` once for each route in the batch. Send only IDs. NOTIFY payloads are capped at 8000 bytes.

### Optional

On a `route_subscribed` notification, skip the fetch if that route's departures were synced within the last few seconds. This matters when a route's lease expires and a new subscriber shows up right away.

## Constants

| Name | Value | Owner |
|---|---|---|
| `LEASE_TTL` | 45s | WS server |
| `HEARTBEAT_INTERVAL` | 15s | WS server |
| `SUBSCRIBE_DEBOUNCE` | 200ms | requester |
| departures tick | 10s | requester |
| channel `route_subscribed` | payload: route UUID | trigger to requester |
| channel `departures_updated` | payload: route UUID | requester to WS server |

`LEASE_TTL` has to be at least 3x `HEARTBEAT_INTERVAL`, so one slow heartbeat does not drop a lease.

## Alternatives rejected

- Polling `route_subscription` faster. Adds latency and constant queries, and is still not immediate.
- Redis pub/sub. Another service to run when Postgres already covers this.
- WS server calling the requester over HTTP. Couples the two processes and needs discovery and retries.
- One row per WebSocket connection. Writes scale with connection churn instead of with routes.
- Logical replication or CDC. Far more than one small table needs.
