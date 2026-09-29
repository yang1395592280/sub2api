import { beforeEach, describe, expect, it } from 'vitest'
import { buildPelicanPreviewSource, getPelicanPreviewNonce } from '../pelicanPreview'

describe('pelicanPreview', () => {
  beforeEach(() => {
    document.head.innerHTML = ''
  })

  it('reads the nonce used by the host page', () => {
    const script = document.createElement('script')
    script.setAttribute('nonce', 'host-nonce')
    document.head.append(script)

    expect(getPelicanPreviewNonce()).toBe('host-nonce')
  })

  it('adds the host nonce to inline scripts without enabling external scripts', () => {
    const source = buildPelicanPreviewSource(
      '<!doctype html><html><head></head><body><script>window.started = true</script><script src="https://example.com/app.js"></script><script nonce="old">window.alsoStarted = true</script></body></html>',
      'host-nonce'
    )

    expect(source).toContain('<script nonce="host-nonce">window.started = true</script>')
    expect(source).toContain('<script src="https://example.com/app.js"></script>')
    expect(source).toContain('<script nonce="host-nonce">window.alsoStarted = true</script>')
  })
})
