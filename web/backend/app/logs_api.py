from __future__ import annotations

from datetime import datetime, timezone

from app import logbuf
from app.agent_client import agent, engine


def _parse_time(value: str) -> datetime | None:
    return logbuf.parse_time(value)


def _format_time(dt: datetime) -> str:
    return logbuf.format_time(dt)


def _entry_after(entry: dict, since: datetime | None) -> bool:
    if since is None:
        return True
    ts = _parse_time(entry.get("time", ""))
    return ts is None or ts > since


def _dedupe(entries: list[dict]) -> list[dict]:
    seen: set[tuple[str, str, str]] = set()
    out: list[dict] = []
    for e in entries:
        key = (e.get("time") or "", e.get("source") or "", e.get("line") or "")
        if key in seen:
            continue
        seen.add(key)
        out.append(e)
    return out


async def list_sources() -> list[dict]:
    sources = [
        {"id": "bytebay-panel", "label": "Panel Web", "group": "bytebay"},
    ]
    try:
        data = await agent.get("/api/v1/logs/sources")
        if isinstance(data, list):
            sources.extend(data)
    except Exception:
        pass
    try:
        data = await engine.get("/api/v1/logs/sources")
        if isinstance(data, list):
            sources.extend(data)
    except Exception:
        pass
    seen: set[str] = set()
    out = []
    for s in sources:
        sid = s.get("id")
        if not sid or sid in seen:
            continue
        seen.add(sid)
        out.append(s)
    return out


async def fetch_entries(since: str = "", sources: str = "") -> dict:
    since_dt = _parse_time(since)
    since_param = _format_time(since_dt) if since_dt else ""
    selected = {p.strip() for p in sources.split(",") if p.strip()} if sources else None
    merged: list[dict] = []

    if selected is None or "bytebay-panel" in selected:
        merged.extend(logbuf.entries_since(since_param))

    agent_sources = []
    engine_sources = []
    try:
        all_sources = await list_sources()
        for s in all_sources:
            sid = s["id"]
            if sid == "bytebay-panel":
                continue
            if sid.startswith("bytebay-engine-proc"):
                engine_sources.append(sid)
            else:
                agent_sources.append(sid)
    except Exception:
        pass

    agent_csv = ",".join(
        sid for sid in agent_sources if selected is None or sid in selected
    )
    if agent_csv or (selected is None and agent_sources):
        try:
            params = [f"since={since_param}" if since_param else "since="]
            if agent_csv:
                params.append(f"sources={agent_csv}")
            data = await agent.get("/api/v1/logs?" + "&".join(params))
            merged.extend(data.get("entries") or [])
        except Exception as exc:
            merged.append(
                {
                    "source": "bytebay-agent",
                    "time": _format_time(datetime.now(timezone.utc)),
                    "line": f"[panel] agent logs indisponibles: {exc}",
                }
            )

    if selected is None or "bytebay-engine-proc" in selected:
        try:
            q = f"/api/v1/logs?since={since_param}" if since_param else "/api/v1/logs?since="
            data = await engine.get(q)
            merged.extend(data.get("entries") or [])
        except Exception as exc:
            merged.append(
                {
                    "source": "bytebay-engine-proc",
                    "time": _format_time(datetime.now(timezone.utc)),
                    "line": f"[panel] engine logs indisponibles: {exc}",
                }
            )

    if since_dt is not None:
        merged = [e for e in merged if _entry_after(e, since_dt)]

    merged = _dedupe(merged)
    merged.sort(key=lambda e: (_parse_time(e.get("time") or "") or datetime.min.replace(tzinfo=timezone.utc), e.get("source") or "", e.get("line") or ""))
    if len(merged) > 3000:
        merged = merged[-3000:]
    return {
        "entries": merged,
        "server_time": _format_time(datetime.now(timezone.utc)),
    }
