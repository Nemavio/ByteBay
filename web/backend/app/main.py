from contextlib import asynccontextmanager
from pathlib import Path
from typing import Any
from urllib.parse import quote
import asyncio
import logging
import mimetypes

mimetypes.add_type("font/woff2", ".woff2")
mimetypes.add_type("image/heic", ".heic")
mimetypes.add_type("image/heif", ".heif")
mimetypes.add_type("image/avif", ".avif")
mimetypes.add_type("image/webp", ".webp")
mimetypes.add_type("video/x-matroska", ".mkv")
mimetypes.add_type("video/quicktime", ".mov")
mimetypes.add_type("video/x-msvideo", ".avi")
mimetypes.add_type("audio/flac", ".flac")
mimetypes.add_type("audio/opus", ".opus")
mimetypes.add_type("audio/aac", ".aac")

from fastapi import Body, Depends, FastAPI, File, HTTPException, Query, UploadFile
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import FileResponse, Response
from fastapi.staticfiles import StaticFiles
from pydantic import BaseModel

from app.acl import check_file_access, is_admin, path_allowed, require_admin, require_web
from app.agent_client import agent, engine
from app.auth import create_access_token, verify_password
from app.database import (
    create_user,
    delete_acl,
    delete_user,
    get_acl_for_user,
    get_db,
    get_user_by_username,
    init_db,
    list_acl,
    list_users,
    pwd_context,
    set_acl,
    update_user,
    users_for_engine,
)
from app import logbuf
from app.logs_api import fetch_entries, list_sources
from app.config import settings
from app.webdav import router as webdav_router

STATIC = Path(__file__).resolve().parent.parent / "static"
log = logging.getLogger("bytebay")


@asynccontextmanager
async def lifespan(app: FastAPI):
    logbuf.install()
    await init_db()
    try:
        import aiosqlite

        async with aiosqlite.connect(settings.db_path) as db:
            db.row_factory = aiosqlite.Row
            passwords = {}
            try:
                state = await engine.get("/api/v1/users/state")
                if not state.get("persisted"):
                    passwords[settings.admin_user] = settings.admin_password
            except Exception:
                passwords[settings.admin_user] = settings.admin_password
            await sync_engine_users(db, passwords)
    except Exception as exc:
        log.warning("engine user sync on startup: %s", exc)
    yield
    await agent.close()
    await engine.close()


app = FastAPI(title="ByteBay", version="0.2.0", lifespan=lifespan)
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(webdav_router)


class LoginForm(BaseModel):
    username: str
    password: str


class UserCreate(BaseModel):
    username: str
    password: str
    web_role: str = "viewer"
    samba_enabled: bool = False
    ftp_enabled: bool = False


class UserUpdate(BaseModel):
    web_role: str | None = None
    samba_enabled: bool | None = None
    ftp_enabled: bool | None = None
    password: str | None = None


class ACLCreate(BaseModel):
    path: str
    username: str
    can_read: bool = True
    can_write: bool = False


class RaidCreate(BaseModel):
    level: str
    devices: list[str]
    raid_devices: int | None = None
    name: str | None = None


class RaidAdd(BaseModel):
    device: str


class ConfirmPassword(BaseModel):
    password: str


class MountCreate(BaseModel):
    name: str
    source: str
    fstype: str = "ext4"
    format: bool = False
    options: str = "defaults"


class NetworkConfig(BaseModel):
    renderer: str = "networkd"
    dns: list[str] = []
    connections: list[dict]


async def sync_engine_users(db, passwords: dict[str, str] | None = None):
    rows = await users_for_engine(db)
    acl_rows = await list_acl(db)
    users = []
    for r in rows:
        pwd = (passwords or {}).get(r["username"], "")
        users.append(
            {
                "username": r["username"],
                "password": pwd,
                "samba": bool(r["samba_enabled"]),
                "ftp": bool(r["ftp_enabled"]),
            }
        )
    acl = [
        {
            "path": a["path"],
            "username": a["username"],
            "can_read": bool(a["can_read"]),
            "can_write": bool(a["can_write"]),
        }
        for a in acl_rows
    ]
    await engine.post("/api/v1/users/sync", {"users": users, "acl": acl})


@app.post("/api/v1/auth/login")
async def login(form: LoginForm, db=Depends(get_db)):
    user = await get_user_by_username(db, form.username)
    if not user or not verify_password(form.password, user["password_hash"]):
        raise HTTPException(401, "Invalid credentials")
    if user["web_role"] == "none":
        raise HTTPException(403, "No web panel access")
    return {"access_token": create_access_token(user["username"]), "token_type": "bearer"}


@app.get("/api/v1/auth/me")
async def me(user=Depends(require_web)):
    return {
        "id": user["id"],
        "username": user["username"],
        "web_role": user["web_role"],
        "is_admin": user["web_role"] == "admin",
        "samba_enabled": bool(user["samba_enabled"]),
        "ftp_enabled": bool(user["ftp_enabled"]),
    }


@app.get("/api/v1/users")
async def users_list(user=Depends(require_admin), db=Depends(get_db)):
    return [dict(r) for r in await list_users(db)]


@app.post("/api/v1/users")
async def users_create(body: UserCreate, user=Depends(require_admin), db=Depends(get_db)):
    if body.web_role not in ("none", "viewer", "admin"):
        raise HTTPException(400, "Invalid web_role")
    try:
        await create_user(
            db, body.username, body.password, body.web_role, body.samba_enabled, body.ftp_enabled
        )
        await sync_engine_users(db, {body.username: body.password})
    except Exception as e:
        raise HTTPException(400, str(e))
    return {"ok": True}


@app.patch("/api/v1/users/{user_id}")
async def users_update(user_id: int, body: UserUpdate, user=Depends(require_admin), db=Depends(get_db)):
    fields = {}
    passwords = {}
    if body.web_role is not None:
        fields["web_role"] = body.web_role
    if body.samba_enabled is not None:
        fields["samba_enabled"] = int(body.samba_enabled)
    if body.ftp_enabled is not None:
        fields["ftp_enabled"] = int(body.ftp_enabled)
    cur = await db.execute("SELECT username, samba_enabled, ftp_enabled FROM users WHERE id = ?", (user_id,))
    row = await cur.fetchone()
    if not row:
        raise HTTPException(404, "User not found")
    if body.samba_enabled and int(body.samba_enabled) and not row["samba_enabled"] and not body.password:
        raise HTTPException(400, "Mot de passe requis pour activer l'accès Samba")
    if body.ftp_enabled and int(body.ftp_enabled) and not row["ftp_enabled"] and not body.password:
        raise HTTPException(400, "Mot de passe requis pour activer l'accès FTP")
    if body.password:
        fields["password_hash"] = pwd_context.hash(body.password)
        passwords[row["username"]] = body.password
    await update_user(db, user_id, **fields)
    await sync_engine_users(db, passwords)
    return {"ok": True}


@app.delete("/api/v1/users/{user_id}")
async def users_delete(user_id: int, user=Depends(require_admin), db=Depends(get_db)):
    if user_id == user["id"]:
        raise HTTPException(400, "Cannot delete yourself")
    await delete_user(db, user_id)
    await sync_engine_users(db)
    return {"ok": True}


@app.get("/api/v1/acl")
async def acl_list(user=Depends(require_admin), db=Depends(get_db)):
    return [
        {
            "id": r["id"],
            "path": r["path"],
            "username": r["username"],
            "can_read": bool(r["can_read"]),
            "can_write": bool(r["can_write"]),
        }
        for r in await list_acl(db)
    ]


@app.post("/api/v1/acl")
async def acl_create(body: ACLCreate, user=Depends(require_admin), db=Depends(get_db)):
    await set_acl(db, body.path, body.username, body.can_read, body.can_write)
    await sync_engine_users(db)
    return {"ok": True}


@app.delete("/api/v1/acl/{acl_id}")
async def acl_delete(acl_id: int, user=Depends(require_admin), db=Depends(get_db)):
    await delete_acl(db, acl_id)
    await sync_engine_users(db)
    return {"ok": True}


@app.get("/api/v1/files")
async def files_list(path: str = "/data", user=Depends(require_web), db=Depends(get_db)):
    await check_file_access(user, db, path)
    return await engine.get(f"/api/v1/files?path={quote(path, safe='/')}")


@app.post("/api/v1/files/mkdir")
async def files_mkdir(body: dict, user=Depends(require_web), db=Depends(get_db)):
    await check_file_access(user, db, body.get("path", ""), write=True)
    return await engine.post("/api/v1/files/mkdir", body)


@app.post("/api/v1/files/upload")
async def files_upload(
    path: str = Query(...),
    file: UploadFile = File(...),
    user=Depends(require_web),
    db=Depends(get_db),
):
    await check_file_access(user, db, path, write=True)
    content = await file.read()
    dest = path.rstrip("/") + "/" + (file.filename or "upload")
    return await engine.post_raw(
        f"/api/v1/files/upload?path={quote(path, safe='/')}&name={quote(file.filename or 'upload', safe='')}",
        content,
        file.content_type or "application/octet-stream",
    )


@app.get("/api/v1/files/download")
async def files_download(path: str, user=Depends(require_web), db=Depends(get_db)):
    await check_file_access(user, db, path)
    qpath = quote(path, safe="/")
    stat = await engine.get(f"/api/v1/files/stat?path={qpath}")
    media_type = stat.get("mime") or mimetypes.guess_type(path)[0] or "application/octet-stream"
    data = await engine.get_raw(f"/api/v1/files/download?path={qpath}")
    return Response(content=data, media_type=media_type)


@app.get("/api/v1/disks")
async def disks(user=Depends(require_web)):
    return await agent.get("/api/v1/disks")


@app.get("/api/v1/disks/{device}/smart")
async def disk_smart(device: str, user=Depends(require_web)):
    return await agent.get(f"/api/v1/disks/{device}/smart")


@app.get("/api/v1/raid")
async def raid_list(user=Depends(require_web)):
    return await agent.get("/api/v1/raid")


@app.get("/api/v1/raid/{name}")
async def raid_detail(name: str, user=Depends(require_web)):
    return await agent.get(f"/api/v1/raid/{name}")


@app.post("/api/v1/raid")
async def raid_create(body: RaidCreate, user=Depends(require_admin)):
    return await agent.post("/api/v1/raid", body.model_dump(exclude_none=True))


@app.get("/api/v1/raid/jobs/{job_id}")
async def raid_job(job_id: str, user=Depends(require_web)):
    return await agent.get(f"/api/v1/raid/jobs/{job_id}")


@app.get("/api/v1/housekeeping")
async def housekeeping(user=Depends(require_web)):
    return await agent.get("/api/v1/housekeeping")


@app.post("/api/v1/housekeeping/recover-raid")
async def housekeeping_recover_raid(body: dict, user=Depends(require_admin)):
    return await agent.post("/api/v1/housekeeping/recover-raid", body)


@app.post("/api/v1/raid/{name}/add")
async def raid_add(name: str, body: RaidAdd, user=Depends(require_admin)):
    return await agent.post(f"/api/v1/raid/{name}/add", body.model_dump())


@app.post("/api/v1/raid/{name}/sync")
async def raid_sync(name: str, body: dict, user=Depends(require_admin)):
    return await agent.post(f"/api/v1/raid/{name}/sync", body)


@app.delete("/api/v1/raid/{name}")
async def raid_stop(name: str, body: ConfirmPassword, user=Depends(require_admin), db=Depends(get_db)):
    if not verify_password(body.password, user["password_hash"]):
        raise HTTPException(403, "Mot de passe incorrect")
    return await agent.delete(f"/api/v1/raid/{name}")


@app.get("/api/v1/mounts")
async def mounts_list(user=Depends(require_web)):
    return await agent.get("/api/v1/mounts")


@app.post("/api/v1/mounts")
async def mounts_create(body: MountCreate, user=Depends(require_admin)):
    return await agent.post("/api/v1/mounts", body.model_dump())


@app.get("/api/v1/mounts/jobs/{job_id}")
async def mounts_job(job_id: str, user=Depends(require_web)):
    return await agent.get(f"/api/v1/mounts/jobs/{job_id}")


@app.delete("/api/v1/mounts/{name}")
async def mounts_delete(name: str, user=Depends(require_admin)):
    return await agent.delete(f"/api/v1/mounts/{name}")


@app.get("/api/v1/network")
async def network_get(user=Depends(require_admin)):
    return await agent.get("/api/v1/network")


@app.put("/api/v1/network")
async def network_put(body: NetworkConfig, user=Depends(require_admin)):
    return await agent.put("/api/v1/network", body.model_dump())


@app.post("/api/v1/network/apply")
async def network_apply(user=Depends(require_admin)):
    return await agent.post("/api/v1/network/apply")


@app.get("/api/v1/volumes")
async def volumes_list(user=Depends(require_web)):
    return await engine.get("/api/v1/volumes")


@app.get("/api/v1/smart")
async def smart_all(user=Depends(require_web)):
    return await agent.get("/api/v1/smart")


@app.get("/api/v1/smart/alerts")
async def smart_alerts(user=Depends(require_web)):
    return await agent.get("/api/v1/smart/alerts")


@app.get("/api/v1/shares")
async def shares_list(user=Depends(require_web)):
    return await engine.get("/api/v1/shares")


@app.put("/api/v1/shares/{kind}")
async def shares_put(kind: str, body: Any = Body(...), user=Depends(require_admin)):
    return await engine.put(f"/api/v1/shares/{kind}", body)


@app.post("/api/v1/shares/apply")
async def shares_apply(user=Depends(require_admin)):
    return await engine.post("/api/v1/shares/apply")


@app.get("/api/v1/logs/sources")
async def logs_sources(user=Depends(require_web)):
    return await list_sources()


@app.get("/api/v1/logs")
async def logs_fetch(
    since: str = "",
    sources: str = "",
    user=Depends(require_web),
):
    return await fetch_entries(since=since, sources=sources)


@app.get("/api/v1/dashboard")
async def dashboard(user=Depends(require_web)):
    host = {}
    services = {}
    agent_ok = False
    engine_ok = False
    try:
        await asyncio.wait_for(agent.get("/health"), timeout=5.0)
        agent_ok = True
        host = await agent.get("/api/v1/dashboard")
    except HTTPException:
        pass
    except Exception:
        pass
    try:
        await asyncio.wait_for(engine.get("/health"), timeout=5.0)
        engine_ok = True
        services = await engine.get("/api/v1/services/status")
    except HTTPException:
        pass
    except Exception:
        pass
    return {
        "platform": {"web": True, "agent": agent_ok, "engine": engine_ok},
        "host": host,
        "services": services,
    }


@app.get("/api/v1/health")
async def health():
    agent_ok = engine_ok = False
    try:
        await asyncio.wait_for(agent.get("/health"), timeout=5.0)
        agent_ok = True
    except Exception:
        pass
    try:
        await asyncio.wait_for(engine.get("/health"), timeout=5.0)
        engine_ok = True
    except Exception:
        pass
    return {"web": True, "agent": agent_ok, "engine": engine_ok}


if STATIC.exists():
    fonts_dir = STATIC / "fonts"
    if fonts_dir.exists():
        app.mount("/fonts", StaticFiles(directory=fonts_dir), name="fonts")
    app.mount("/assets", StaticFiles(directory=STATIC / "assets"), name="assets")

    @app.get("/{full_path:path}")
    async def spa(full_path: str):
        if full_path.startswith("api/"):
            raise HTTPException(404)
        index = STATIC / "index.html"
        if index.exists():
            return FileResponse(
                index,
                headers={"Cache-Control": "no-cache, no-store, must-revalidate", "Pragma": "no-cache"},
            )
        raise HTTPException(404)
