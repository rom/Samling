import { html, t, Page } from '../lib.js';

// Online help (placeholder).
export function Help() {
  return html`<${Page} title=${t('nav.help')}><//>`;
}
