import httpx
from fastapi import HTTPException

from app.config import settings


def _client(socket: str, token: str) -> httpx.AsyncClient:
    headers = {}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    return httpx.AsyncClient(
        transport=httpx.AsyncHTTPTransport(uds=socket),
        base_url="http://bytebay",
        headers=headers,
        timeout=120.0,
    )


def _agent_error(response: httpx.Response) -> HTTPException:
    try:
        body = response.json()
        msg = body.get("error") or body.get("detail") or response.text
    except Exception:
        msg = response.text or response.reason_phrase
    return HTTPException(response.status_code, msg)


class SocketClient:
    def __init__(self, socket: str, token: str = ""):
        self.client = _client(socket, token)

    async def get(self, path: str):
        r = await self.client.get(path)
        if r.is_error:
            raise _agent_error(r)
        return r.json()

    async def get_raw(self, path: str) -> bytes:
        r = await self.client.get(path)
        if r.is_error:
            raise _agent_error(r)
        return r.content

    async def post(self, path: str, body: dict | None = None):
        r = await self.client.post(path, json=body or {})
        if r.is_error:
            raise _agent_error(r)
        return r.json()

    async def post_raw(self, path: str, data: bytes, content_type: str):
        r = await self.client.post(path, content=data, headers={"Content-Type": content_type})
        if r.is_error:
            raise _agent_error(r)
        return r.json()

    async def put_stream(self, path: str, stream, content_type: str = "application/octet-stream"):
        r = await self.client.put(
            path,
            content=stream,
            headers={"Content-Type": content_type},
        )
        if r.is_error:
            raise _agent_error(r)
        if r.status_code == 204:
            return None
        return r.json() if r.content else None

    async def delete_path(self, path: str):
        r = await self.client.delete(path)
        if r.is_error:
            raise _agent_error(r)
        if r.status_code == 204:
            return None
        return r.json() if r.content else None

    async def put(self, path: str, body):
        r = await self.client.put(path, json=body)
        if r.is_error:
            raise _agent_error(r)
        return r.json()

    async def delete(self, path: str, body: dict | None = None):
        r = await self.client.request("DELETE", path, json=body)
        if r.is_error:
            raise _agent_error(r)
        if r.status_code == 204:
            return None
        return r.json()

    async def close(self):
        await self.client.aclose()


agent = SocketClient(settings.agent_socket, settings.agent_token)
engine = SocketClient(settings.engine_socket, settings.engine_token)
