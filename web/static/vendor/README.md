# Vendored client libraries

| File | Source | Licence |
|------|--------|---------|
| `htm-preact-standalone.js` | npm package `htm` 3.1.1, file `preact/standalone.module.js` (htm bundled with Preact and its hooks) | htm: Apache-2.0, Preact: MIT |

The file is committed unmodified so the client needs no build step and no
third-party requests at run time. To update it, replace the file with the
same path from a newer `htm` release and run `make e2e`.
