import base64
import http.client
import json
import os
import threading
import unittest
from http.server import ThreadingHTTPServer
from unittest.mock import patch

import renderer

SVG = '<svg xmlns="http://www.w3.org/2000/svg" width="960" height="640"><circle cx="100" cy="100" r="30" fill="orange"><animate attributeName="cx" values="100;800;100" dur="3s" repeatCount="indefinite"/></circle></svg>'


class RendererTest(unittest.TestCase):
    def test_rejects_scripts_resources_and_css_escapes(self):
        for fragment in ('<script/>', '<image href="https://example.com"/>', '<path onload="x"/>', '<style>@import "https://example.com";</style>', '<style>path{fill:u\\72l(https://example.com)}</style>'):
            with self.subTest(fragment=fragment), self.assertRaises(ValueError):
                renderer.inspect_svg(SVG.replace('</svg>', fragment + '</svg>'))

    def test_http_auth_body_limit_and_no_arbitrary_fields(self):
        server = ThreadingHTTPServer(('127.0.0.1', 0), renderer.Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            with patch.dict(os.environ, {'QUALITY_RENDERER_TOKEN': 'local-test-token'}):
                for headers, body, expected in [
                    ({}, {'svg': SVG}, 401),
                    ({'Authorization': 'Bearer local-test-token'}, {'url': 'http://example.com'}, 400),
                    ({'Authorization': 'Bearer local-test-token', 'Content-Length': str(renderer.MAX_BODY + 1)}, {}, 413),
                ]:
                    conn = http.client.HTTPConnection('127.0.0.1', server.server_port, timeout=3)
                    conn.request('POST', '/render', body=json.dumps(body), headers=headers)
                    response = conn.getresponse()
                    self.assertEqual(response.status, expected)
                    response.read()
                    conn.close()
        finally:
            server.shutdown()
            server.server_close()
            thread.join()

    @unittest.skipUnless(os.environ.get('QUALITY_RENDERER_BROWSER_TEST') == '1', 'explicit local Chromium test')
    def test_real_four_frame_animation(self):
        frames = renderer.render(SVG)
        self.assertEqual(len(frames), 4)
        self.assertEqual([f['time'] for f in frames], sorted(f['time'] for f in frames))
        self.assertGreater(len({f['png'] for f in frames}), 1)
        for frame in frames:
            self.assertTrue(base64.b64decode(frame['png']).startswith(b'\x89PNG\r\n\x1a\n'))


if __name__ == '__main__':
    unittest.main()
