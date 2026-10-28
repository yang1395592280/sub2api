"""Private SVG renderer adapted from manxue-ai render_frames.py (Apache-2.0).

Source: third_party/manxue-ai, revision 5da79674417dd0748682f09b7b7467efd1968ea8.
Changes: authenticated HTTP wrapper, explicit SVG allowlist, bounded subprocess.
No account credentials, arbitrary URLs, HTML or JavaScript are accepted.
"""
import base64
import hmac
import json
import os
import re
import signal
import subprocess
import sys
import threading
import xml.etree.ElementTree as ET
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

MAX_SVG = 64 * 1024
MAX_BODY = 512 * 1024
MAX_OUTPUT = 8 * 1024 * 1024
SLOT = threading.BoundedSemaphore(1)
ALLOWED = set("svg g defs symbol use path rect circle ellipse line polyline polygon text tspan textPath title desc style linearGradient radialGradient stop clipPath mask pattern marker filter feGaussianBlur feOffset feBlend feColorMatrix feMerge feMergeNode feFlood feComposite feDropShadow animate animateTransform animateMotion mpath set".split())


def safe_css(value):
    if re.search(r"javascript\s*:|expression\s*\(|/\*|\\", value, re.I):
        return False
    value = re.sub(r"url\s*\(\s*[\"']?#[a-zA-Z_][\w:.-]*[\"']?\s*\)", "", value, flags=re.I)
    value = re.sub(r"@(?:-webkit-)?keyframes\b", "", value, flags=re.I)
    return "@" not in value and not re.search(r"url\s*\(", value, re.I)


def inspect_svg(svg):
    if not isinstance(svg, str) or len(svg.encode()) > MAX_SVG or re.search(r"<!DOCTYPE|<!ENTITY|<\?", svg, re.I):
        raise ValueError("Invalid SVG")
    root = ET.fromstring(svg)
    if root.tag not in ("svg", "{http://www.w3.org/2000/svg}svg"):
        raise ValueError("Invalid SVG root")
    count = 0
    for el in root.iter():
        count += 1
        local = el.tag.rsplit("}", 1)[-1]
        if count > 5000 or local not in ALLOWED or (el.tag.startswith("{") and not el.tag.startswith("{http://www.w3.org/2000/svg}")):
            raise ValueError("Unsafe SVG element")
        for attr, value in el.attrib.items():
            name = attr.rsplit("}", 1)[-1].lower()
            if name.startswith("on") or name in ("src", "base") or (name == "href" and not re.fullmatch(r"#[a-zA-Z_][\w:.-]*", value)) or not safe_css(value):
                raise ValueError("Unsafe SVG attribute")
            if name == "attributename" and (value.lower().startswith("on") or "href" in value.lower() or ":" in value or value in ("src", "style")):
                raise ValueError("Unsafe SVG animation")
        if not safe_css(el.text or ""):
            raise ValueError("Unsafe SVG style")
    return svg


def capture(svg):
    from playwright.sync_api import sync_playwright
    inspect_svg(svg)
    with sync_playwright() as p:
        browser = p.chromium.launch(args=["--disable-dev-shm-usage"])
        context = browser.new_context(viewport={"width": 960, "height": 640}, service_workers="block", accept_downloads=False)
        context.route("**/*", lambda route: route.abort())
        page = context.new_page()
        page.set_default_timeout(8000)
        page.route("http://render.invalid/scene.svg", lambda route: route.fulfill(
            content_type="image/svg+xml", body=svg,
            headers={"Content-Security-Policy": "default-src 'none'; style-src 'unsafe-inline'; script-src 'none'; frame-src 'none'"}))
        page.goto("http://render.invalid/scene.svg", wait_until="load")
        cycle = page.evaluate("""() => {
          const svg = document.querySelector('svg');
          svg.style.setProperty('width', '960px', 'important');
          svg.style.setProperty('height', '640px', 'important');
          svg.style.setProperty('overflow', 'hidden', 'important');
          svg.style.setProperty('background', 'white', 'important');
          svg.pauseAnimations();
          const durations = [...svg.querySelectorAll('animate,animateTransform,animateMotion')].map(a => {
            try { return a.getSimpleDuration(); } catch { return 0; }
          });
          document.getAnimations().forEach(a => { a.pause(); durations.push(a.effect.getTiming().duration / 1000); });
          const usable = durations.filter(v => Number.isFinite(v) && v >= .5 && v <= 10);
          return usable.length ? Math.min(...usable) : 3;
        }""")
        frames = []
        for fraction in (0, .23, .51, .79):
            seconds = cycle * fraction
            page.evaluate("""seconds => {
              document.querySelector('svg').setCurrentTime(seconds);
              document.getAnimations().forEach(a => { a.pause(); a.currentTime = seconds * 1000; });
            }""", seconds)
            frames.append({"time": seconds, "png": base64.b64encode(page.screenshot(type="png", animations="allow")).decode()})
        context.close()
        browser.close()
        return frames


def render(svg):
    inspect_svg(svg)
    process = subprocess.Popen([sys.executable, str(Path(__file__).resolve()), "--worker"],
                               stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, start_new_session=True)
    try:
        raw, _ = process.communicate(svg.encode(), timeout=30)
        if process.returncode != 0 or len(raw) > MAX_OUTPUT:
            raise ValueError("Rendering failed")
        return json.loads(raw)
    finally:
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        process.wait()


class Handler(BaseHTTPRequestHandler):
    def setup(self):
        super().setup()
        self.connection.settimeout(10)

    def log_message(self, *_args):
        pass  # Do not log generated content or authorization headers.

    def respond(self, code, value):
        raw = json.dumps(value).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.send_header("Connection", "close")
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self):
        self.respond(200 if self.path == "/health" else 404, {"status": "ready"})

    def do_POST(self):
        if self.path != "/render":
            return self.respond(404, {"error": "not_found"})
        token = os.environ.get("QUALITY_RENDERER_TOKEN", "")
        if token and not hmac.compare_digest(self.headers.get("Authorization", ""), "Bearer " + token):
            return self.respond(401, {"error": "unauthorized"})
        try:
            length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            length = 0
        if length <= 0 or length > MAX_BODY or self.headers.get("Transfer-Encoding"):
            return self.respond(413, {"error": "body_limit"})
        if not SLOT.acquire(blocking=False):
            return self.respond(429, {"error": "renderer_busy"})
        try:
            request = json.loads(self.rfile.read(length))
            if set(request) != {"svg"}:
                raise ValueError("Invalid request")
            self.respond(200, render(request["svg"]))
        except (ValueError, TypeError, ET.ParseError):
            self.respond(400, {"error": "invalid_svg"})
        except Exception:
            self.respond(503, {"error": "render_failed"})
        finally:
            SLOT.release()


if __name__ == "__main__":
    if "--worker" in sys.argv:
        svg_input = sys.stdin.buffer.read(MAX_SVG + 1)
        print(json.dumps(capture(svg_input.decode())))
    else:
        ThreadingHTTPServer((os.environ.get("QUALITY_RENDERER_BIND", "0.0.0.0"), 8091), Handler).serve_forever()
