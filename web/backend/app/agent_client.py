import httpx

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


class SocketClient:
    def __init__(self, socket: str, token: str = ""):
        self.client = _client(socket, token)

    async def get(self, path: str):
        r = await self.client.get(path)
        r.raise_for_status()
        return r.json()

    async def get_raw(self, path: str) -> bytes:
        r = await self.client.get(path)
        r.raise_for_status()
        return r.content

    async def post(self, path: str, body: dict | None = None):
        r = await self.client.post(path, json=body or {})
        r.raise_for_status()
        return r.json()

    async def post_raw(self, path: str, data: bytes, content_type: str):
        r = await self.client.post(path, content=data, headers={"Content-Type": content_type})
        r.raise_for_status()
        return r.json()

    async def put(self, path: str, body):
        r = await self.client.put(path, json=body)
        r.raise_for_status()
        return r.json()

    async def delete(self, path: str):
        r = await self.client.delete(path)
        r.raise_for_status()
        return r.json()

    async def close(self):
        await self.client.aclose()


agent = SocketClient(settings.agent_socket, settings.agent_token)
engine = SocketClient(settings.engine_socket, settings.engine_token)
