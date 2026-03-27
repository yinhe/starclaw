"""
🐝 Drone API Server (lightweight Python)
Provides /stats, /records, /harvest/:source endpoints.
Runs on port 8110 on Server C.

Start: python3 -m scripts.api_server
"""

import json
import os
import sys
import subprocess
import threading
import time
from http.server import HTTPServer, BaseHTTPRequestHandler
from datetime import datetime
from urllib.parse import urlparse

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

PORT = int(os.getenv("DRONE_PORT", "8110"))

# In-memory state
records = []
running = {}
lock = threading.Lock()


class DroneHandler(BaseHTTPRequestHandler):

    def do_GET(self):
        path = urlparse(self.path).path.rstrip("/")

        if path == "/health":
            self._json(200, {"service": "drone-api", "status": "ok"})

        elif path == "/stats":
            with lock:
                total_c = sum(r.get("collected", 0) for r in records if r["status"] == "completed")
                total_m = sum(r.get("morphed", 0) for r in records if r["status"] == "completed")
                total_i = sum(r.get("imported", 0) for r in records if r["status"] == "completed")
                completed = sum(1 for r in records if r["status"] == "completed")
                failed = sum(1 for r in records if r["status"] == "failed")
                running_list = list(running.keys())
            self._json(200, {
                "total_runs": len(records),
                "completed": completed,
                "failed": failed,
                "running": running_list,
                "total_collected": total_c,
                "total_morphed": total_m,
                "total_imported": total_i,
            })

        elif path == "/records":
            with lock:
                recs = list(reversed(records[-50:]))
            self._json(200, {"records": recs, "total": len(recs)})

        else:
            self._json(404, {"error": "not found"})

    def do_POST(self):
        path = urlparse(self.path).path.rstrip("/")

        if path.startswith("/harvest/"):
            source = path.split("/harvest/")[-1]
            self._trigger(source)
        elif path == "/harvest":
            # Trigger all known sources
            sources = ["gpt_prompts", "awesome_gpts"]
            started = []
            for s in sources:
                with lock:
                    if s not in running:
                        running[s] = True
                        started.append(s)
                        threading.Thread(target=self._run_harvest, args=(s,), daemon=True).start()
            self._json(202, {"message": f"harvest started: {len(started)} sources", "sources": started})
        else:
            self._json(404, {"error": "not found"})

    def _trigger(self, source):
        with lock:
            if source in running:
                self._json(409, {"error": f"{source} already running"})
                return
            running[source] = True

        threading.Thread(target=self._run_harvest, args=(source,), daemon=True).start()
        self._json(202, {"message": f"harvest started: {source}", "source": source})

    def _run_harvest(self, source):
        start = time.time()
        rec = {
            "id": f"{source}_{datetime.utcnow().strftime('%Y%m%d_%H%M%S')}",
            "source": source,
            "status": "running",
            "collected": 0, "morphed": 0, "imported": 0,
            "duration": "", "error": "",
            "started_at": datetime.utcnow().isoformat() + "Z",
        }

        try:
            env = os.environ.copy()
            env["DRONE_DATA_DIR"] = os.getenv("DRONE_DATA_DIR", "/data/harvest")
            env["DRONE_CLAW_API"] = os.getenv("DRONE_CLAW_API", "https://app.starclaw.me")
            env["DRONE_SECRET"] = os.getenv("DRONE_SECRET", "")

            result = subprocess.run(
                [sys.executable, "-m", "scripts.worker", source],
                cwd=os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
                env=env, capture_output=True, text=True, timeout=600,
            )

            output = result.stdout + result.stderr
            rec["duration"] = f"{time.time() - start:.1f}s"

            # Parse output for numbers
            for line in output.split("\n"):
                if "collected" in line and "→" in line:
                    parts = line.split("→")
                    for p in parts:
                        p = p.strip()
                        if "collected" in p:
                            try: rec["collected"] = int(p.split()[0])
                            except: pass
                        elif "morphed" in p:
                            try: rec["morphed"] = int(p.split()[0])
                            except: pass
                        elif "imported" in p:
                            try: rec["imported"] = int(p.split()[0])
                            except: pass

            if result.returncode == 0:
                rec["status"] = "completed"
            else:
                rec["status"] = "failed"
                rec["error"] = output[-200:] if output else "exit code " + str(result.returncode)

        except subprocess.TimeoutExpired:
            rec["status"] = "failed"
            rec["error"] = "timeout (600s)"
            rec["duration"] = "600s"
        except Exception as e:
            rec["status"] = "failed"
            rec["error"] = str(e)
            rec["duration"] = f"{time.time() - start:.1f}s"
        finally:
            with lock:
                running.pop(source, None)
                records.append(rec)
                if len(records) > 100:
                    del records[:-100]

        print(f"[drone-api] {rec['status']}: {source} collected={rec['collected']} morphed={rec['morphed']} imported={rec['imported']} ({rec['duration']})")

    def _json(self, code, data):
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type, Authorization")
        self.end_headers()
        self.wfile.write(json.dumps(data, ensure_ascii=False).encode())

    def do_OPTIONS(self):
        self._json(200, {})

    def log_message(self, format, *args):
        print(f"[drone-api] {args[0]}")


if __name__ == "__main__":
    print(f"🐝 Drone API starting on :{PORT}")
    server = HTTPServer(("0.0.0.0", PORT), DroneHandler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\n🐝 Drone API stopped")
