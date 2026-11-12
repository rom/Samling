// Reading text from a picture, on the device: the picture never leaves the
// browser. The recogniser (Tesseract, see vendor/README.md) is large, so it
// is loaded only when somebody actually scans something.
const base = '/vendor/ocr';

export async function recognize(image, onProgress) {
  const { default: T } = await import('../vendor/ocr/tesseract.esm.min.js');
  const worker = await T.createWorker(['swe', 'eng'], 1, {
    workerPath: base + '/worker.min.js',
    corePath: base + '/tesseract-core-simd-lstm.wasm.js',
    langPath: base,
    gzip: true,
    workerBlobURL: false, // the content security policy only allows scripts from this server
    logger: (m) => { if (onProgress && m.status === 'recognizing text') onProgress(m.progress); },
  });
  try {
    const { data } = await worker.recognize(image);
    return (data.text || '').replace(/[ \t]+\n/g, '\n').replace(/\n{3,}/g, '\n\n').trim();
  } finally {
    await worker.terminate();
  }
}
