from __future__ import annotations

from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
from typing import Callable

from tiktok_trend_shop.config import AppConfig


def health_payload(config: AppConfig) -> dict[str, object]:
    return {
        "status": "ok",
        "env": config.env,
        "storage_backend": config.storage_backend,
        "regions": config.regions,
    }


class HealthHandler(BaseHTTPRequestHandler):
    config_factory: Callable[[], AppConfig] = AppConfig.from_env

    def do_GET(self) -> None:
        if self.path != "/health":
            self.send_response(404)
            self.end_headers()
            return
        payload = json.dumps(health_payload(self.config_factory())).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, format: str, *args: object) -> None:
        return


def create_server(host: str = "127.0.0.1", port: int = 8080) -> ThreadingHTTPServer:
    return ThreadingHTTPServer((host, port), HealthHandler)
