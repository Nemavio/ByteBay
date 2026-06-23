"""WebDAV front-end for ByteBay file explorer (proxies to engine with ACL checks)."""

from __future__ import annotations

from email.utils import formatdate
from urllib.parse import quote, unquote, urlparse
import mimetypes
import xml.etree.ElementTree as ET

from fastapi import APIRouter, Depends, HTTPException, Request, Response

from app.acl import check_file_access, require_web
from app.agent_client import engine
from app.database import get_db

router = APIRouter()

DAV_PREFIX = "/api/v1/webdav"
DAV_NS = "DAV:"
XML_NS = "http://www.w3.org/XML/1998/namespace"


def fs_path_from_dav(dav_path: str) -> str:
    raw = unquote(dav_path or "").strip("/")
    if not raw:
        return "/volumes"
    path = "/" + raw
    if not (path == "/data" or path.startswith("/data/") or path == "/volumes" or path.startswith("/volumes/")):
        raise HTTPException(400, "invalid path")
    return path


def dav_href(fs_path: str) -> str:
    if fs_path == "/":
        return DAV_PREFIX + "/"
    return DAV_PREFIX + quote(fs_path, safe="/")


def _prop_xml(name: str, value: str | None, ns: str = DAV_NS) -> ET.Element:
    el = ET.Element(f"{{{ns}}}{name}")
    if value is not None:
        el.text = value
    return el


def _response_for_entry(fs_path: str, ent: dict) -> ET.Element:
    resp = ET.Element(f"{{{DAV_NS}}}response")
    href = ET.SubElement(resp, f"{{{DAV_NS}}}href")
    href.text = dav_href(fs_path)
    propstat = ET.SubElement(resp, f"{{{DAV_NS}}}propstat")
    prop = ET.SubElement(propstat, f"{{{DAV_NS}}}prop")
    name = ent.get("name") or fs_path.rsplit("/", 1)[-1]
    prop.append(_prop_xml("displayname", name))
    if ent.get("is_dir"):
        rt = ET.SubElement(prop, f"{{{DAV_NS}}}resourcetype")
        ET.SubElement(rt, f"{{{DAV_NS}}}collection")
    else:
        prop.append(_prop_xml("resourcetype", None))
        prop.append(_prop_xml("getcontentlength", str(ent.get("size") or 0)))
        if ent.get("mime"):
            prop.append(_prop_xml("getcontenttype", ent["mime"]))
    if ent.get("mod_time"):
        try:
            from datetime import datetime

            dt = datetime.fromisoformat(ent["mod_time"].replace("Z", "+00:00"))
            prop.append(_prop_xml("getlastmodified", formatdate(dt.timestamp(), usegmt=True)))
        except Exception:
            pass
    status = ET.SubElement(propstat, f"{{{DAV_NS}}}status")
    status.text = "HTTP/1.1 200 OK"
    return resp


def build_multistatus(entries: list[tuple[str, dict]]) -> bytes:
    root = ET.Element(f"{{{DAV_NS}}}multistatus")
    for fs_path, ent in entries:
        root.append(_response_for_entry(fs_path, ent))
    body = ET.tostring(root, encoding="utf-8", xml_declaration=True)
    return body


def destination_fs_path(request: Request, header: str) -> str:
    dest = (header or "").strip()
    if not dest:
        raise HTTPException(400, "Destination required")
    if dest.startswith("http://") or dest.startswith("https://"):
        parsed = urlparse(dest)
        dest = parsed.path
    if dest.startswith(DAV_PREFIX):
        dest = dest[len(DAV_PREFIX) :]
    return fs_path_from_dav(dest)


async def get_entry_stat(fs_path: str) -> dict:
    try:
        return await engine.get(f"/api/v1/files/stat?path={quote(fs_path, safe='/')}")
    except HTTPException as exc:
        if exc.status_code not in (400, 404):
            raise
    if fs_path in ("/data", "/volumes"):
        return {"name": fs_path.strip("/") or "volumes", "path": fs_path, "is_dir": True, "size": 0}
    parent = fs_path.rsplit("/", 1)[0] or "/volumes"
    listed = await engine.get(f"/api/v1/files?path={quote(parent, safe='/')}")
    if isinstance(listed, list):
        for ent in listed:
            if ent.get("path") == fs_path:
                return ent
    raise HTTPException(404, "not found")


async def propfind_entries(fs_path: str, depth: str) -> list[tuple[str, dict]]:
    depth = (depth or "1").strip()
    if depth not in ("0", "1"):
        depth = "1"

    stat = await get_entry_stat(fs_path)
    out: list[tuple[str, dict]] = [(fs_path, stat)]

    if depth == "1" and stat.get("is_dir"):
        listed = await engine.get(f"/api/v1/files?path={quote(fs_path, safe='/')}")
        if isinstance(listed, list):
            for ent in listed:
                if ent.get("name") == "..":
                    continue
                out.append((ent["path"], ent))
    return out


@router.api_route("/api/v1/webdav", methods=["OPTIONS"])
@router.api_route("/api/v1/webdav/{path:path}", methods=["OPTIONS"])
async def webdav_options(_path: str = ""):
    return Response(
        status_code=200,
        headers={
            "DAV": "1, 2",
            "Allow": "OPTIONS, GET, HEAD, PUT, DELETE, MKCOL, PROPFIND, MOVE",
            "MS-Author-Via": "DAV",
        },
    )


@router.api_route("/api/v1/webdav", methods=["PROPFIND"])
@router.api_route("/api/v1/webdav/{path:path}", methods=["PROPFIND"])
async def webdav_propfind(
    request: Request,
    path: str = "",
    user=Depends(require_web),
    db=Depends(get_db),
):
    fs_path = fs_path_from_dav(path)
    await check_file_access(user, db, fs_path)
    depth = request.headers.get("Depth", "1")
    try:
        entries = await propfind_entries(fs_path, depth)
    except HTTPException:
        raise
    except Exception as exc:
        raise HTTPException(400, str(exc)) from exc
    return Response(
        content=build_multistatus(entries),
        media_type="application/xml; charset=utf-8",
        status_code=207,
    )


@router.api_route("/api/v1/webdav", methods=["GET", "HEAD"])
@router.api_route("/api/v1/webdav/{path:path}", methods=["GET", "HEAD"])
async def webdav_get(
    request: Request,
    path: str = "",
    user=Depends(require_web),
    db=Depends(get_db),
):
    fs_path = fs_path_from_dav(path)
    await check_file_access(user, db, fs_path)
    stat = await get_entry_stat(fs_path)
    if stat.get("is_dir"):
        raise HTTPException(405, "Method Not Allowed")
    data = await engine.get_raw(f"/api/v1/files/download?path={quote(fs_path, safe='/')}")
    headers = {}
    mime = stat.get("mime") or mimetypes.guess_type(fs_path)[0]
    if mime:
        headers["Content-Type"] = mime
    if stat.get("mod_time"):
        try:
            from datetime import datetime

            dt = datetime.fromisoformat(stat["mod_time"].replace("Z", "+00:00"))
            headers["Last-Modified"] = formatdate(dt.timestamp(), usegmt=True)
        except Exception:
            pass
    if request.method == "HEAD":
        headers["Content-Length"] = str(len(data))
        return Response(status_code=200, headers=headers)
    return Response(content=data, headers=headers)


@router.api_route("/api/v1/webdav", methods=["PUT"])
@router.api_route("/api/v1/webdav/{path:path}", methods=["PUT"])
async def webdav_put(
    request: Request,
    path: str = "",
    user=Depends(require_web),
    db=Depends(get_db),
):
    fs_path = fs_path_from_dav(path)
    await check_file_access(user, db, fs_path, write=True)
    ct = request.headers.get("content-type", "application/octet-stream")

    async def stream():
        async for chunk in request.stream():
            yield chunk

    try:
        await engine.put_stream(
            f"/api/v1/files?path={quote(fs_path, safe='/')}",
            stream(),
            ct,
        )
    except HTTPException:
        raise
    except Exception as exc:
        raise HTTPException(400, str(exc)) from exc
    return Response(status_code=201)


@router.api_route("/api/v1/webdav", methods=["MKCOL"])
@router.api_route("/api/v1/webdav/{path:path}", methods=["MKCOL"])
async def webdav_mkcol(
    path: str = "",
    user=Depends(require_web),
    db=Depends(get_db),
):
    fs_path = fs_path_from_dav(path)
    await check_file_access(user, db, fs_path, write=True)
    try:
        await engine.post("/api/v1/files/mkdir", {"path": fs_path})
    except HTTPException as exc:
        if exc.status_code == 400 and "exist" in str(exc.detail).lower():
            raise HTTPException(405, "Collection already exists") from exc
        raise
    return Response(status_code=201)


@router.api_route("/api/v1/webdav", methods=["DELETE"])
@router.api_route("/api/v1/webdav/{path:path}", methods=["DELETE"])
async def webdav_delete(
    path: str = "",
    user=Depends(require_web),
    db=Depends(get_db),
):
    fs_path = fs_path_from_dav(path)
    if fs_path in ("/data", "/volumes"):
        raise HTTPException(403, "cannot delete root")
    await check_file_access(user, db, fs_path, write=True)
    try:
        await engine.delete_path(f"/api/v1/files?path={quote(fs_path, safe='/')}")
    except HTTPException:
        raise
    except Exception as exc:
        raise HTTPException(400, str(exc)) from exc
    return Response(status_code=204)


@router.api_route("/api/v1/webdav", methods=["MOVE"])
@router.api_route("/api/v1/webdav/{path:path}", methods=["MOVE"])
async def webdav_move(
    request: Request,
    path: str = "",
    user=Depends(require_web),
    db=Depends(get_db),
):
    fs_path = fs_path_from_dav(path)
    dest = destination_fs_path(request, request.headers.get("Destination", ""))
    await check_file_access(user, db, fs_path, write=True)
    await check_file_access(user, db, dest, write=True)
    try:
        await engine.post("/api/v1/files/move", {"from": fs_path, "to": dest})
    except HTTPException:
        raise
    except Exception as exc:
        raise HTTPException(400, str(exc)) from exc
    return Response(status_code=201)
