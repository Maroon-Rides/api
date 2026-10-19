# POI search

Status: research

## Goal

Trip planning takes an origin and destination as coordinates. Search turns what the user types ("ZACH", "msc", "Northgate garage", "HEB") into a name and a lat/lon. Today the user picks stops from the local `stop` table. Later, stops leave search and only places remain: buildings, apartments, businesses, landmarks, parking.

## Decided

- POIs live in Postgres only. They are not synced to the app, because trip planning needs the network anyway.
- A requester job (gocron, `apps/requester/jobs`) refreshes them from the sources.
- The gateway serves a search endpoint. The app calls it with a debounced query.

## Sources

Service area: the "Tight Routes" bbox in `geospatial/BOUNDS.md`, `-96.525584,30.510059,-96.200881,30.720020`.

### TAMU FCOR ArcGIS (gis.tamu.edu)

Public, no auth. `ArcGIS 11.5`, `maxRecordCount` 2000, `Query` allowed. Counts were queried on 2026-10-19.

| Layer                | URL (under `https://gis.tamu.edu/arcgis/rest/services/FCOR/`) | Rows                                         | Useful fields                                                                                    |
| -------------------- | -------------------------------------------------------------- | -------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| University Building  | `TAMU_Basemap_20261008/MapServer/2`                            | 1802 (776 with `IsBldg='Y' AND status='1'`) | `Number` (e.g. `0518`), `BldgAbbrev` (`ZACH`), `BldgName`, `Address`, `Type`, `Latitude`, `Longitude` |
| Garage Parking       | `TAMU_Basemap_20261008/MapServer/0`                            | 10                                           | `FAC_CODE` (`CCG`), `LotName` ("Central Campus Garage"), `LotType`                               |
| Surface Parking      | `TAMU_Basemap_20261008/MapServer/9`                            | 340                                          | `LotName` ("Lot 094"), `LotType`                                                                 |
| Point Of Interest    | `MapInfo_20190529/MapServer/0`                                 | 21                                           | `Name` ("Bonfire Memorial"), `Type`                                                              |
| Building Entrance    | `ADA_120717/MapServer/0`                                       | 1524                                         | `BldgNumber`, `BldgAbbrev`, `Category`, door flags                                               |

Notes:

- `TAMU_Basemap_*` is re-published under a new name every few months: `_05152026`, `_060826`, `_20261008`, each with a different date format. Old ones stay up for now (`_060826` still answers, 1799 rows). The job cannot hardcode one. It has to list `FCOR?f=json` and pick the newest, or read the name from config.
- Text fields are fixed-width and padded with spaces (`"Bright Building   ..."`, `"COLLEGE STATION   ..."`). Trim them, and title-case the city.
- 63 of the 776 active buildings have no name or no lat/lon.
- Parking layers have no lat/lon fields, and `returnCentroid` returns nothing. Fetch polygons with `outSR=4326` and compute the centroid in Go.
- Entrances can later replace the building centroid as the destination point.
- No license: `copyrightText` and the item-info license field are both empty. The data is public state data, but there is no written grant.

### OpenStreetMap (Overpass)

One Overpass query on 2026-10-19 for `nwr[name]` with `amenity|shop|tourism|leisure|building|office|healthcare|craft|landuse=residential` inside the bbox:

| Category        | Named features |
| --------------- | -------------- |
| amenity         | 880 (154 restaurants, 134 parking, 123 fast food) |
| building        | 599            |
| shop            | 283            |
| landuse (resid.)| 210            |
| leisure         | 199            |
| tourism / office / other | 163   |
| **Total**       | **2334**       |

- Campus buildings carry the TAMU codes: `short_name=ZACH`, `ref=0518`, `old_name=Zachry Engineering Center`. 94 features have `short_name`, 20 `alt_name`, 53 `official_name`.
- `ref` on 80 buildings is the TAMU `Number`, so OSM and TAMU rows join exactly on that key.
- 240 apartment complexes or residential areas are named. Apartments are a large share of riders' trips.
- Size: the full JSON is 803 KB. Name, lat/lon, and category alone come to 182 KB, or 49 KB gzipped.
- License: ODbL. Per the OSMF [Geocoding Guideline](https://osmfoundation.org/wiki/Licence/Community_Guidelines/Geocoding_-_Guideline):
  - A geocoder that uses OSM must credit "© OpenStreetMap contributors", and so must an app built on that geocoder.
  - Individual results (name plus lat/lon) carry no share-alike.
  - Our table is a systematic extract of OSM's named POIs for a city, which makes it a Derivative Database. Using it publicly means we must offer the OSM-derived rows under ODbL on request.
  - If OSM rows are stored unmodified and separately from TAMU rows (by `source`), the TAMU rows stay outside share-alike, because the combination is a Collective Database. This is my reading of the license, not legal advice.

### Overture Maps places

- License: CDLA-Permissive-2.0 ([release notes](https://docs.overturemaps.org/release/page/15)). No share-alike. Data comes from Meta and Microsoft.
- Count for the bbox was not measured; that needs duckdb or the `overturemaps` CLI, and neither is installed. Overture likely covers businesses better than OSM. Its known weaknesses are closed businesses and duplicates, so filter on `confidence`.
- It has no TAMU building codes. Use it for businesses only, if OSM coverage turns out to be thin.

### Commercial geocoders

| Provider      | Can store results?                                                                                         | Source                                                                                                                                                                                         |
| ------------- | ---------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Google Places | `place_id` only, indefinitely. Lat/lng at most 30 days. No other pre-fetching or storage. Google logo required when results show without a Google map. | [Places policies](https://developers.google.com/maps/documentation/places/web-service/policies), [service terms](https://cloud.google.com/archive/maps-platform/terms/maps-service-terms-20250501) |
| Mapbox        | Only with `permanent=true`, which costs more per request and needs a card on file.                          | [Temporary vs permanent](https://docs.mapbox.com/help/dive-deeper/understand-temporary-vs-permanent-geocoding/)                                                                                 |

- AggieSpirit's own trip planner uses Google Places autocomplete. We cannot copy that data into Postgres.
- Google or Mapbox could serve as a live fallback for queries with no local hit. That adds per-request cost and the logo rule, and sends each query to a third party.

## Dataset size

| Set                                           | Rows (est.) |
| --------------------------------------------- | ----------- |
| TAMU buildings (active, named)                | ~710        |
| TAMU garages + surface lots + campus POIs     | ~370        |
| OSM named POIs, minus TAMU-ref duplicates     | ~2250       |
| **Total**                                     | **~3.3k**   |

That is under 1 MB in Postgres. A sequential scan is cheap at this size, so indexes matter more for headroom than for speed.

## Refresh job

- One job per source, e.g. `JobPoiTamu` and `JobPoiOsm`, each with `gocron.DurationJob(24 * time.Hour)`. TAMU changes every few months and OSM daily. Weekly would also be fine. Daily catches renames sooner, and it is one Overpass query plus about 5 ArcGIS pages.
- Overpass: one query per run is far inside its usage policy. Send a project `User-Agent`.
- Upsert by `(source, sourceId)`:
  1. Fetch everything, page with `resultOffset`, and normalize: trim fields, compute centroids, collect aliases.
  2. In one transaction, `INSERT ... ON CONFLICT (source, sourceId) DO UPDATE`, then `DELETE FROM poi WHERE source = ? AND sourceId <> ALL(?)`.
  3. Guard: if the fetch returns fewer than `MIN_REFRESH_RATIO` (e.g. 0.8) times the current row count for that source, skip the delete and fail the job. An empty or broken upstream must not wipe the table.
- Dedupe: skip OSM features whose `ref` matches a TAMU `Number`, and fold their `short_name`/`alt_name` into the TAMU row's aliases. TAMU wins for campus buildings. OSM "Harvey R. Bright Building" becomes an alias of TAMU "Bright Building".
- `sourceId`:
  - TAMU: `Number` for buildings, `FAC_CODE` or `LotName` for parking
  - OSM: `n123`, `w123`, `r123`
- Hand-curated aliases ("Sbisa", "the Rec", "Commons") come from a `poi_alias` table or a checked-in YAML file that the job merges in. Upstream data has none of these.

## Postgres search

`postgres:18-alpine` ships `pg_trgm` and `earthdistance` as contrib extensions. PostGIS is not in the image.

### pg_trgm vs tsvector

| Need                                   | pg_trgm                                                                   | tsvector                                                                                 |
| -------------------------------------- | ------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| Typing a prefix ("memor")              | `word_similarity`, `<%`                                                   | `to_tsquery('memor:*')`                                                                  |
| Typos ("zachy", "memorail")            | yes                                                                       | no                                                                                       |
| Mid-word ("gate" in "Northgate")       | yes, `ILIKE '%gate%'` is indexed                                          | no                                                                                       |
| Codes ("ZACH", "MSC")                  | exact alias match, plus trigram                                           | exact lexeme only                                                                        |
| Stemming                               | none, and none is wanted for names                                        | English stemming mangles names ("Evans" → "evan")                   |
| Index                                  | GIN `gin_trgm_ops` (filters, `ILIKE`) or GiST (also KNN `<->` ordering)   | GIN                                                                                      |

`pg_trgm` covers every need. tsvector adds nothing for short place names. Facts are from the [pg_trgm docs](https://www.postgresql.org/docs/current/pgtrgm.html):

- default `word_similarity_threshold` 0.6, `similarity_threshold` 0.3
- KNN ordering (`<->`, `<<->`) is supported by GiST only

### Ranking

Score each candidate on text match first, then distance:

| Match                                                 | Base score                              |
| ----------------------------------------------------- | --------------------------------------- |
| `q` equals an alias or code (`lower(q) = ANY(aliases)`) | 1.0                                   |
| a word in `searchText` starts with `q`                | 0.9                                     |
| otherwise                                             | `word_similarity(q, searchText)`        |

- Proximity, when `lat`/`lon` are sent: subtract `DISTANCE_WEIGHT * ln(1 + km)`.
- Distance uses plain equirectangular math in SQL: `111.32 * sqrt((lat-$1)^2 + ((lon-$2)*cos(radians($1)))^2)`. Error is under 0.1% at city scale. No extension is needed, because only candidates are scored.
- Queries of 1 or 2 characters have too few trigrams. Use a prefix match on `searchText` and aliases only.
- Weights are named Go constants, covered by table tests ("msc" → Memorial Student Center first, "zach" → Zachry, "zachy" → Zachry).

Query shape:

```sql
SELECT id, name, kind, lat, lon, <score> AS score
FROM poi
WHERE searchText ILIKE '%' || $q || '%' OR $q <% searchText OR lower($q) = ANY(aliases)
ORDER BY score DESC
LIMIT $limit;
```

- Indexes: GIN `gin_trgm_ops` on `searchText`, and GIN on `aliases`.
- Expected latency is low single-digit ms in Postgres for about 3k rows. This is an estimate; confirm with `EXPLAIN ANALYZE` on seeded data. End-to-end latency is dominated by the network.
- `searchText` is `lower(unaccent(name || ' ' || array_to_string(aliases, ' ')))`, computed by the job, not a generated column. `array_to_string` is not immutable.

## Endpoint

`GET /api/places/search?q=zach&lat=30.6187&lon=-96.3365&limit=10`

| Param      | Rule                                                        |
| ---------- | ----------------------------------------------------------- |
| `q`        | required, 1–64 chars after trim                             |
| `lat`,`lon`| optional; both or neither; must fall inside the wider map bbox |
| `limit`    | default 10, max 25                                          |

Response:

```json
{ "results": [ { "id": "0199...", "name": "Zachry Engineering Education Complex", "kind": "BUILDING", "subtitle": "ZACH · 125 Spence St", "lat": 30.621252, "lon": -96.340241, "distanceMeters": 310 } ] }
```

- `kind` is an enum: `BUILDING`, `PARKING`, `HOUSING`, `BUSINESS`, `LANDMARK`.
- The app shows "© OpenStreetMap contributors" in the search UI for OSM rows.

Client:

- Debounce 150 ms after the last keystroke. The server answers in ms, so a longer delay only adds lag. Fire right away on submit.
- Cancel the in-flight request with `AbortController` on each new query, and drop responses that arrive out of order.
- Round `lat`/`lon` to 3 decimals (~100 m) before sending. This costs nothing in ranking and keeps the exact location off the server.

Server:

- Do not log `q` together with coordinates.
- Responses for requests without `lat`/`lon` can be cached: `Cache-Control: public, max-age=300`.
- Load: one indexed query per debounced keystroke. That is small next to the vehicle websocket traffic.

## Recommendation

- **Approach:** server-side search in Postgres with `pg_trgm` only (no tsvector, no PostGIS), refreshed daily by a requester job.
- **Sources:**
  - TAMU FCOR buildings and parking: authoritative names and codes on campus.
  - OSM via Overpass: businesses, apartments, landmarks, and code aliases.
  - Overture: deferred until OSM coverage of businesses proves thin.
  - Google and Mapbox are out, because their terms forbid storing results.
- **Schema:**

```text
poi
  id          uuid pk default uuidv7()
  source      text   not null   -- TAMU | OSM | MANUAL
  sourceId    text   not null   -- unique (source, sourceId)
  kind        text   not null   -- BUILDING | PARKING | HOUSING | BUSINESS | LANDMARK
  name        text   not null
  aliases     text[] not null default '{}'   -- lowercased: zach, msc, old names
  subtitle    text                            -- code and/or address
  lat         double precision not null
  lon         double precision not null
  searchText  text   not null                 -- GIN gin_trgm_ops
  refreshedAt timestamptz not null
```

- **Code:**
  - migration enabling `pg_trgm` and `unaccent`, plus the `poi` model (not `sync.Syncable`)
  - `apps/requester`: `PoiService` with TAMU and OSM fetchers, and two jobs
  - `apps/gateway`: places controller → places service → poi repository
  - OpenAPI-generated client in the app
- **Rollout:** ship the endpoint next to local stop search. Drop stops from search once POI results are good enough.

## Open questions

1. TAMU data has no license. Ask TAMU Facilities/GIS for written permission and a stable service URL, or treat it as public data and read the newest `TAMU_Basemap_*` from config?
2. Is a public dump of the OSM rows on request acceptable for ODbL, or should OSM be limited to aliases and a few categories to stay "insubstantial"?
3. Path name: `/api/places/search` (user-facing) or `/api/pois/search` (matches the table)?
4. Should stop names stay searchable as POIs (`kind = STOP`) after the switch, or be dropped completely?
5. Who curates the manual aliases ("Sbisa", "the Rec"), and where do they live: YAML in the repo, or a table with an admin path?
6. Is a Google Places fallback ever wanted for misses? It costs money, sends queries to Google, and requires the logo.
