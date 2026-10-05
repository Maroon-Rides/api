# Changelog

## [1.2.2](https://github.com/Maroon-Rides/api/compare/v1.2.1...v1.2.2) (2026-10-05)


### Bug Fixes

* drop directions missing from base data ([9c75891](https://github.com/Maroon-Rides/api/commit/9c75891b1c6e08a5272ee756501ef24cb593b8e7))

## [1.2.1](https://github.com/Maroon-Rides/api/compare/v1.2.0...v1.2.1) (2026-10-02)


### Bug Fixes

* unique stops per direction ([6f97ea2](https://github.com/Maroon-Rides/api/commit/6f97ea233376494a22235fb7adb6442767fa14a0))

## [1.2.0](https://github.com/Maroon-Rides/api/compare/v1.1.0...v1.2.0) (2026-10-01)


### Features

* log failed job runs from the scheduler ([c43140c](https://github.com/Maroon-Rides/api/commit/c43140cc2d079d83e36d7b17d4e1e9bd06dda1e4))


### Bug Fixes

* skip tls verification for the gtfs feed ([aa254c7](https://github.com/Maroon-Rides/api/commit/aa254c7c349108cff99e9db9751deb059d743343))

## [1.1.0](https://github.com/Maroon-Rides/api/compare/v1.0.0...v1.1.0) (2026-10-01)


### Features

* default direction stop timepoints to false and keep them across route syncs ([a85194f](https://github.com/Maroon-Rides/api/commit/a85194f782b672eac2d3ef1be85c5e8dd0e85249))
* sync stop timepoints hourly from the tamu gtfs feed ([008ffd3](https://github.com/Maroon-Rides/api/commit/008ffd373a516eafe4ee5dcf6a531bb1b5960a49))


### Bug Fixes

* start job scheduler after migrations apply ([10ba24e](https://github.com/Maroon-Rides/api/commit/10ba24ee95c9de83a027b0e22627f01113c2f6c0))

## 1.0.0 (2026-10-01)


### Features

* ci ([fdad42b](https://github.com/Maroon-Rides/api/commit/fdad42bd5b96c81f82eb3b3beeb9ad67689f185a))
* cors ([afc832c](https://github.com/Maroon-Rides/api/commit/afc832c5b9631f90bc42d8f7ab5344b28dc114ec))
* initial commit ([b3caf6b](https://github.com/Maroon-Rides/api/commit/b3caf6b039d2be0717db4ec2dac70a33151023ad))
* sync api completion, websocket evets, and load testing ([d0561b9](https://github.com/Maroon-Rides/api/commit/d0561b9afd776937ec6cc24b56fbdcd059e5768c))
* testing and per commit builds ([a425dd7](https://github.com/Maroon-Rides/api/commit/a425dd7148923fe7a26b6e4a9aa34bc155bf5d76))
