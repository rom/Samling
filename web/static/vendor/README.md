# Vendored client libraries

| File | Source | Licence |
|------|--------|---------|
| `htm-preact-standalone.js` | npm package `htm` 3.1.1, file `preact/standalone.module.js` (htm bundled with Preact and its hooks) | htm: Apache-2.0, Preact: MIT |

The file is committed unmodified so the client needs no build step and no
third-party requests at run time. To update it, replace the file with the
same path from a newer `htm` release and run `make e2e`.

## Text recognition (`ocr/`)

Used by "Scan text with the camera". The text is recognised in the browser;
nothing is sent to a third party. The files are loaded only when somebody
scans a picture.

| File | Source | Licence |
|------|--------|---------|
| `ocr/tesseract.esm.min.js`, `ocr/worker.min.js` | npm package `tesseract.js` 7.0.0, directory `dist/` | Apache-2.0 |
| `ocr/tesseract-core-simd-lstm.wasm.js` | npm package `tesseract.js-core` 6.1.2 (Tesseract compiled to WebAssembly, LSTM engine, SIMD) | Apache-2.0 |
| `ocr/eng.traineddata.gz`, `ocr/swe.traineddata.gz` | npm packages `@tesseract.js-data/eng` and `@tesseract.js-data/swe`, directory `4.0.0_best_int/` | Packages: MIT; the trained data comes from the Tesseract project: Apache-2.0 |

The files are committed unmodified. The script files refer to source maps
that are not included; browsers ignore the missing maps. To add a language,
add its `*.traineddata.gz` here and its code to the list in `js/ocr.js`. To
update, replace the files with the same paths from newer releases (keep the
`tesseract.js` and `tesseract.js-core` versions matched as that project
documents) and run `make e2e`, which scans a picture.

Compiling WebAssembly requires `'wasm-unsafe-eval'` in the content security
policy (see `docs/security.md`).
