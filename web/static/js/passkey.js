// Passkeys (WebAuthn) in the browser: converts between the JSON the server
// speaks and the binary structures of navigator.credentials.
import { api } from './lib.js';

const dec = (s) => {
  const b = atob(s.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(s.length / 4) * 4, '='));
  return Uint8Array.from(b, (c) => c.charCodeAt(0));
};
const enc = (buf) => {
  let s = '';
  new Uint8Array(buf).forEach((x) => { s += String.fromCharCode(x); });
  return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
};

// Passkeys need a secure context (https, or localhost during development).
export const passkeysSupported = () => !!(window.isSecureContext && window.PublicKeyCredential && navigator.credentials);

export async function registerPasskey(password, name) {
  const { ceremony, options } = await api('POST', '/me/passkeys/begin', { password });
  options.challenge = dec(options.challenge);
  options.user.id = dec(options.user.id);
  (options.excludeCredentials || []).forEach((c) => { c.id = dec(c.id); });
  const cred = await navigator.credentials.create({ publicKey: options });
  const r = cred.response;
  return api('POST', '/me/passkeys/finish', {
    ceremony, name,
    credential: {
      id: cred.id, rawId: enc(cred.rawId), type: cred.type,
      authenticatorAttachment: cred.authenticatorAttachment || undefined,
      clientExtensionResults: cred.getClientExtensionResults ? cred.getClientExtensionResults() : {},
      response: {
        attestationObject: enc(r.attestationObject), clientDataJSON: enc(r.clientDataJSON),
        transports: r.getTransports ? r.getTransports() : undefined,
      },
    },
  });
}

export async function signInWithPasskey() {
  const { ceremony, options } = await api('POST', '/auth/passkey/begin');
  options.challenge = dec(options.challenge);
  (options.allowCredentials || []).forEach((c) => { c.id = dec(c.id); });
  const cred = await navigator.credentials.get({ publicKey: options });
  const r = cred.response;
  return api('POST', '/auth/passkey/finish', {
    ceremony,
    credential: {
      id: cred.id, rawId: enc(cred.rawId), type: cred.type,
      authenticatorAttachment: cred.authenticatorAttachment || undefined,
      clientExtensionResults: cred.getClientExtensionResults ? cred.getClientExtensionResults() : {},
      response: {
        authenticatorData: enc(r.authenticatorData), clientDataJSON: enc(r.clientDataJSON), signature: enc(r.signature),
        userHandle: r.userHandle ? enc(r.userHandle) : undefined,
      },
    },
  });
}
