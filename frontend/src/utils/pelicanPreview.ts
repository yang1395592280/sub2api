export const PELICAN_PREVIEW_WIDTH = 1280
export const PELICAN_PREVIEW_HEIGHT = 720

const previewCsp = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; media-src data: blob:; connect-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'"

export function getPelicanPreviewNonce(): string | undefined {
  if (typeof document === 'undefined') return undefined
  const script = document.querySelector<HTMLScriptElement>('script[nonce]')
  return script?.getAttribute('nonce') || script?.nonce || undefined
}

function escapeAttribute(value: string): string {
  return value.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

function addInlineScriptNonce(html: string, nonce: string | undefined): string {
  if (!nonce) return html
  const escapedNonce = escapeAttribute(nonce)
  return html.replace(/<script\b([^>]*)>/gi, (tag, attributes: string) => {
    // External scripts remain subject to the source allowlist and are never granted the page nonce.
    if (/\bsrc\s*=\s*/i.test(attributes)) return tag
    if (/\bnonce\s*=\s*/i.test(attributes)) {
      return `<script${attributes.replace(/\bnonce\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)/i, `nonce="${escapedNonce}"`)}>`
    }
    return `<script nonce="${escapedNonce}"${attributes}>`
  })
}

export function buildPelicanPreviewSource(html: string, nonce = getPelicanPreviewNonce()): string {
  const meta = `<meta http-equiv="Content-Security-Policy" content="${previewCsp}">`
  // Put the policy before any generated element, even if its <head> is late.
  const doctype = /^\s*<!doctype[^>]*>/i
  const source = doctype.test(html)
    ? html.replace(doctype, match => `${match}${meta}`)
    : `${meta}${html}`
  return addInlineScriptNonce(source, nonce)
}
