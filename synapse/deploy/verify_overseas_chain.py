#!/usr/bin/env python3
import json
import time
import urllib.request
import urllib.error

BASE = "http://127.0.0.1:8096"


def post(path: str, payload: dict, headers: dict | None = None, timeout: int = 60):
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        BASE + path,
        data=data,
        headers={"Content-Type": "application/json", **(headers or {})},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            body = resp.read().decode("utf-8", errors="replace")
            return resp.status, body
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8", errors="replace")
        return e.code, body


def main():
    email = f"e2e_{int(time.time())}@star-ai.net"
    password = "StarAi!123456"

    status, body = post(
        "/auth/register",
        {"email": email, "password": password, "name": "e2e"},
        timeout=30,
    )
    print(f"register_status={status}")
    if status != 201:
        print(body[:500])
        return

    reg = json.loads(body)
    api_key = reg.get("api_key", {}).get("key", "")
    if not api_key:
        print("missing_api_key_from_register")
        return

    status, body = post(
        "/v1/chat/completions",
        {
            "model": "openai/gpt-4o-mini",
            "messages": [{"role": "user", "content": "Reply with OK only."}],
            "stream": False,
        },
        headers={"Authorization": f"Bearer {api_key}"},
        timeout=120,
    )
    print(f"chat_status={status}")
    print(body[:1000])


if __name__ == "__main__":
    main()
