from __future__ import annotations

import logging
from collections import deque
from datetime import datetime, timezone

_MAX = 800
_buffer: deque[dict] = deque(maxlen=_MAX)
_installed = False


def parse_time(value: str) -> datetime | None:
    if not value:
        return None
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None


def format_time(dt: datetime) -> str:
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt.astimezone(timezone.utc).isoformat()


class RingBufferHandler(logging.Handler):
    def emit(self, record: logging.LogRecord) -> None:
        try:
            msg = self.format(record)
        except Exception:
            msg = record.getMessage()
        _buffer.append(
            {
                "source": "bytebay-panel",
                "time": format_time(datetime.fromtimestamp(record.created, tz=timezone.utc)),
                "line": msg,
            }
        )


def install() -> None:
    global _installed
    if _installed:
        return
    handler = RingBufferHandler()
    handler.setFormatter(logging.Formatter("%(levelname)s %(name)s: %(message)s"))
    root = logging.getLogger()
    root.addHandler(handler)
    if root.level == logging.NOTSET:
        root.setLevel(logging.INFO)
    _installed = True


def entries_since(since_iso: str = "") -> list[dict]:
    since = parse_time(since_iso)
    if since is None:
        return list(_buffer)
    out = []
    for e in _buffer:
        ts = parse_time(e["time"])
        if ts is None or ts > since:
            out.append(e)
    return out
