const previewCsp = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; media-src data: blob:; connect-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'"

export function buildPelicanPreviewSource(html: string): string {
  const meta = `<meta http-equiv="Content-Security-Policy" content="${previewCsp}">`
  // Put the policy before any generated element, even if its <head> is late.
  const doctype = /^\s*<!doctype[^>]*>/i
  return doctype.test(html)
    ? html.replace(doctype, match => `${match}${meta}`)
    : `${meta}${html}`
}
