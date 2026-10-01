import { check } from 'k6';
import http from 'k6/http';
import { Counter, Rate, Trend } from 'k6/metrics';
import type { Options } from 'k6/options';
import { clearInterval, setInterval, setTimeout } from 'k6/timers';
import { WebSocket } from 'k6/websockets';

const BASE_URL = __ENV.BASE_URL ?? 'http://localhost:3000';
const SYNC_URL = `${BASE_URL}/api/sync/stream`;
const WS_URL = `${BASE_URL.replace(/^http/, 'ws')}/api/ws`;
const SYNC_TIMETABLES = __ENV.SYNC_TIMETABLES !== 'false';

// Session counts are read off the production graph, which buckets sessions every 10 minutes.
const GRAPH_BUCKET = '10m';
const GRAPH_BUCKET_SECONDS = 600;
const PEAK_SUSTAINED_SESSIONS = 2500;
const PEAK_SPIKE_SESSIONS = 3050;
const LOAD_MULTIPLIER = 3;

const FULL_SYNC_SHARE = 0.3;
const SESSION_SECONDS = 180;
const SYNC_HEADROOM_SECONDS = 30;
const INITIAL_DATA_BUDGET_MS = 2000;
// The gateway drops a client that sends nothing for 60 seconds.
const PING_INTERVAL_MS = 20_000;

// A rider looks at one route for a while, backs out, then opens another.
const ROUTE_HOP_MIN_MS = 60_000;
const ROUTE_HOP_MAX_MS = 120_000;
const ROUTE_HOP_PAUSE_MS = 5_000;

// Each time falls between two of the dev database's change batches, so each returns a different partial sync.
// They go stale as the requester writes more, and the gateway resets any older than 30 days.
const PARTIAL_SYNC_CHECKPOINTS = [
  { label: 'yesterday', syncedAt: '2026-09-29T23:00:00Z', weight: 0.2 },
  { label: 'overnight', syncedAt: '2026-09-30T08:00:00Z', weight: 0.5 },
  { label: 'current', syncedAt: '2026-09-30T13:00:00Z', weight: 0.3 },
];

const ACKED_ENTITY_TYPES = [
  'RouteV1',
  'RouteDeleteV1',
  'DirectionV1',
  'DirectionDeleteV1',
  'StopV1',
  'StopDeleteV1',
  'DirectionStopV1',
  'DirectionStopDeleteV1',
  'AlertV1',
  'AlertDeleteV1',
  'AlertDirectionV1',
  'AlertDirectionDeleteV1',
  ...(SYNC_TIMETABLES ? ['TimetableV1', 'TimetableDeleteV1'] : []),
  'SyncCompleteV1',
];

const SYNC_PROTOCOL_VERSION = 1;

const SUSTAINED_RATE = PEAK_SUSTAINED_SESSIONS * LOAD_MULTIPLIER;
const SPIKE_RATE = PEAK_SPIKE_SESSIONS * LOAD_MULTIPLIER;
const CONCURRENT_SESSIONS_AT_SPIKE = Math.ceil(
  (SPIKE_RATE / GRAPH_BUCKET_SECONDS) * (SESSION_SECONDS + SYNC_HEADROOM_SECONDS),
);

const SYNC_TYPES = [
  'RoutesV1',
  'DirectionsV1',
  'StopsV1',
  'DirectionStopsV1',
  'AlertsV1',
  'AlertDirectionsV1',
  ...(SYNC_TIMETABLES ? ['TimetablesV1'] : []),
];

const SYNC_LINE_TYPES = {
  ROUTE: 'RouteV1',
  SYNC_COMPLETE: 'SyncCompleteV1',
} as const;

const WS_MESSAGE_TYPES = {
  SUBSCRIBE: 'subscribe',
  UNSUBSCRIBE: 'unsubscribe',
  PING: 'ping',
  VEHICLES: 'vehicles',
  DEPARTURES: 'departures',
  ERROR: 'error',
} as const;

const SYNC_KINDS = {
  SETUP: 'setup',
  FULL: 'full',
  PARTIAL: 'partial',
} as const;

type SyncKind = (typeof SYNC_KINDS)[keyof typeof SYNC_KINDS];

const SUBSCRIBE_KINDS = {
  FIRST: 'first',
  HOP: 'hop',
} as const;

type SubscribeKind = (typeof SUBSCRIBE_KINDS)[keyof typeof SUBSCRIBE_KINDS];

// The gateway encodes `type` first, so a prefix match finds route lines without parsing the whole stream.
const ROUTE_LINE_PREFIX = `{"type":"${SYNC_LINE_TYPES.ROUTE}"`;
const ERROR_MESSAGE_PREFIX = `{"type":"${WS_MESSAGE_TYPES.ERROR}"`;
const WS_READY_STATE_OPEN = 1;

interface SyncStreamLine {
  type: string;
  ack: string;
  data: unknown;
}

interface SyncRouteV1 {
  id: string;
  active: boolean;
}

interface SetupData {
  routeIds: string[];
}

interface LiveDataMessage {
  type: string;
  routeId: string;
}

interface Subscription {
  routeId: string;
  kind: SubscribeKind;
  subscribedAt: number;
  awaiting: Set<string>;
}

const initialData = new Trend('ws_initial_data', true);
const initialDataOnTime = new Rate('ws_initial_data_on_time');
const wsErrors = new Counter('ws_errors');
const droppedSessions = new Counter('ws_dropped_sessions');

export const options: Options = {
  scenarios: {
    sessions: {
      executor: 'ramping-arrival-rate',
      timeUnit: GRAPH_BUCKET,
      startRate: 0,
      preAllocatedVUs: CONCURRENT_SESSIONS_AT_SPIKE,
      maxVUs: CONCURRENT_SESSIONS_AT_SPIKE * 2,
      // The ramp is longer than a session so concurrent websockets reach steady state before the hold.
      stages: [
        { target: SUSTAINED_RATE, duration: '5m' },
        { target: SUSTAINED_RATE, duration: '15m' },
        { target: SPIKE_RATE, duration: '1m' },
        { target: SPIKE_RATE, duration: '5m' },
        { target: 0, duration: '1m' },
      ],
      gracefulStop: `${SESSION_SECONDS + SYNC_HEADROOM_SECONDS}s`,
    },
  },
  thresholds: {
    ws_initial_data: [`p(95)<${INITIAL_DATA_BUDGET_MS}`],
    ws_initial_data_on_time: ['rate>0.99'],
    http_req_failed: ['rate<0.01'],
    checks: ['rate>0.99'],
    // Empty thresholds make k6 report these sub-metrics in the summary.
    [`http_req_duration{sync:${SYNC_KINDS.FULL}}`]: [],
    [`http_req_duration{sync:${SYNC_KINDS.PARTIAL}}`]: [],
    ...Object.fromEntries(
      PARTIAL_SYNC_CHECKPOINTS.map((checkpoint) => [`http_req_duration{checkpoint:${checkpoint.label}}`, []]),
    ),
    [`ws_initial_data{subscribe:${SUBSCRIBE_KINDS.FIRST}}`]: [],
    [`ws_initial_data{subscribe:${SUBSCRIBE_KINDS.HOP}}`]: [],
  },
};

export function setup(): SetupData {
  const routeIds = routeIdsIn(sync([], SYNC_KINDS.SETUP));
  if (routeIds.length === 0) {
    throw new Error('setup sync returned no active routes; is the requester running?');
  }
  return { routeIds };
}

export default function (data: SetupData) {
  const routeIds =
    Math.random() < FULL_SYNC_SHARE ? routeIdsIn(sync([], SYNC_KINDS.FULL)) : partialSync(data);
  holdLiveSession(routeIds.length > 0 ? routeIds : data.routeIds);
}

// A returning client already holds its routes, so a partial sync only has to return what changed.
function partialSync(data: SetupData): string[] {
  const checkpoint = pickWeighted(PARTIAL_SYNC_CHECKPOINTS);
  const ackID = uuidv7At(Date.parse(checkpoint.syncedAt));
  sync(
    ACKED_ENTITY_TYPES.map((type) => `${type}|${ackID}`),
    SYNC_KINDS.PARTIAL,
    { checkpoint: checkpoint.label },
  );
  return data.routeIds;
}

function routeIdsIn(body: string): string[] {
  const routes = body
    .split('\n')
    .filter((line) => line.startsWith(ROUTE_LINE_PREFIX))
    .map((line) => (JSON.parse(line) as SyncStreamLine).data as SyncRouteV1);
  return activeRouteIds(routes);
}

function sync(acks: string[], kind: SyncKind, tags: Record<string, string> = {}): string {
  const res = http.post(SYNC_URL, JSON.stringify({ protocol: SYNC_PROTOCOL_VERSION, types: SYNC_TYPES, acks }), {
    headers: { 'Content-Type': 'application/json' },
    tags: { sync: kind, ...tags },
  });
  const body = typeof res.body === 'string' ? res.body : '';

  check(res, {
    'sync status is 200': (r) => r.status === 200,
    'sync stream completes': () => body.includes(`"type":"${SYNC_LINE_TYPES.SYNC_COMPLETE}"`),
  });
  return body;
}

function holdLiveSession(routeIds: string[]) {
  const ws = new WebSocket(WS_URL);
  let current: Subscription | undefined;
  let opened = false;
  let sessionEnded = false;
  let nextHop: ReturnType<typeof setTimeout> | undefined;
  let sessionEnd: ReturnType<typeof setTimeout> | undefined;
  let pinger: ReturnType<typeof setInterval> | undefined;

  const send = (message: object) => {
    if (ws.readyState === WS_READY_STATE_OPEN) {
      ws.send(JSON.stringify(message));
    }
  };

  const stopTimers = () => {
    clearTimeout(nextHop);
    clearTimeout(sessionEnd);
    clearInterval(pinger);
  };

  const subscribe = (routeId: string, kind: SubscribeKind) => {
    const subscription: Subscription = {
      routeId,
      kind,
      subscribedAt: Date.now(),
      awaiting: new Set([WS_MESSAGE_TYPES.VEHICLES, WS_MESSAGE_TYPES.DEPARTURES]),
    };
    current = subscription;
    send({ type: WS_MESSAGE_TYPES.SUBSCRIBE, routeId });

    setTimeout(
      () => initialDataOnTime.add(subscription.awaiting.size === 0, { subscribe: subscription.kind }),
      INITIAL_DATA_BUDGET_MS,
    );
    nextHop = setTimeout(() => hop(routeId), randomBetween(ROUTE_HOP_MIN_MS, ROUTE_HOP_MAX_MS));
  };

  const hop = (fromRouteId: string) => {
    send({ type: WS_MESSAGE_TYPES.UNSUBSCRIBE });
    current = undefined;
    nextHop = setTimeout(() => subscribe(pickOther(routeIds, fromRouteId), SUBSCRIBE_KINDS.HOP), ROUTE_HOP_PAUSE_MS);
  };

  ws.onopen = () => {
    opened = true;
    subscribe(pick(routeIds), SUBSCRIBE_KINDS.FIRST);

    pinger = setInterval(() => send({ type: WS_MESSAGE_TYPES.PING }), PING_INTERVAL_MS);
    sessionEnd = setTimeout(() => {
      sessionEnded = true;
      stopTimers();
      ws.close();
    }, SESSION_SECONDS * 1000);
  };

  ws.onclose = () => {
    stopTimers();
    if (opened && !sessionEnded) {
      droppedSessions.add(1);
    }
  };

  ws.onmessage = (event: { data: string }) => {
    if (event.data.startsWith(ERROR_MESSAGE_PREFIX)) {
      wsErrors.add(1);
      return;
    }
    const subscription = current;
    if (subscription === undefined || subscription.awaiting.size === 0) {
      return;
    }

    // Updates for the route the rider just left can still be in flight.
    const message = JSON.parse(event.data) as LiveDataMessage;
    if (message.routeId !== subscription.routeId) {
      return;
    }
    if (subscription.awaiting.delete(message.type) && subscription.awaiting.size === 0) {
      initialData.add(Date.now() - subscription.subscribedAt, { subscribe: subscription.kind });
    }
  };

  ws.onerror = () => {
    wsErrors.add(1);
    if (!opened) {
      initialDataOnTime.add(false, { subscribe: SUBSCRIBE_KINDS.FIRST });
    }
  };
}

function activeRouteIds(routes: SyncRouteV1[]): string[] {
  return routes.filter((route) => route.active).map((route) => route.id);
}

function pick<T>(items: T[]): T {
  return items[Math.floor(Math.random() * items.length)];
}

function pickOther<T>(items: T[], current: T): T {
  const others = items.filter((item) => item !== current);
  return others.length > 0 ? pick(others) : current;
}

function pickWeighted<T extends { weight: number }>(items: T[]): T {
  let remaining = Math.random() * items.reduce((total, item) => total + item.weight, 0);
  return items.find((item) => (remaining -= item.weight) < 0) ?? items[items.length - 1];
}

function randomBetween(min: number, max: number): number {
  return min + Math.random() * (max - min);
}

// The gateway only compares ack ids byte by byte, so a client-made uuidv7 stands in for one the server sent at that time.
function uuidv7At(epochMs: number): string {
  const time = epochMs.toString(16).padStart(12, '0');
  const random = (digits: number) =>
    Array.from({ length: digits }, () => Math.floor(Math.random() * 16).toString(16)).join('');
  const variant = (8 + Math.floor(Math.random() * 4)).toString(16);
  return `${time.slice(0, 8)}-${time.slice(8)}-7${random(3)}-${variant}${random(3)}-${random(12)}`;
}
