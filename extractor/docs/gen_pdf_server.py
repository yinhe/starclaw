"""Tiny HTTP server that serves the HTML file, then Playwright prints it to PDF."""
import http.server
import threading
import time
import os
import sys

PORT = 18765
HTML_DIR = os.path.dirname(os.path.abspath(__file__))

class Handler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *a, **kw):
        super().__init__(*a, directory=HTML_DIR, **kw)
    def log_message(self, *a):
        pass

server = http.server.HTTPServer(("127.0.0.1", PORT), Handler)
t = threading.Thread(target=server.serve_forever, daemon=True)
t.start()
print(f"Serving on http://127.0.0.1:{PORT}")
print(f"Open http://127.0.0.1:{PORT}/AI_TRADING_ARCHITECTURE.html to view")
print("Use Playwright to print PDF from this URL, then Ctrl+C to stop.")

try:
    while True:
        time.sleep(1)
except KeyboardInterrupt:
    server.shutdown()
