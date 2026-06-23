from fastapi import Depends, HTTPException

from app.auth import get_current_user
from app.database import get_acl_for_user

WEB_ROLES = ("none", "viewer", "admin")


def can_access_web(user) -> bool:
    role = user["web_role"] if "web_role" in user.keys() else "admin"
    return role in ("viewer", "admin")


def is_admin(user) -> bool:
    role = user["web_role"] if "web_role" in user.keys() else "admin"
    return role == "admin"


async def require_web(user=Depends(get_current_user)):
    if not can_access_web(user):
        raise HTTPException(403, "No web access")
    return user


async def require_admin(user=Depends(get_current_user)):
    if not is_admin(user):
        raise HTTPException(403, "Admin only")
    return user


def path_allowed(acl_rows, path: str, write: bool = False) -> bool:
    path = path.rstrip("/") or "/volumes"
    roots = ("/data", "/volumes")
    for row in acl_rows:
        base = row["path"].rstrip("/")
        if path == base or path.startswith(base + "/"):
            if write:
                return bool(row["can_read"] and row["can_write"])
            return bool(row["can_read"])
    return False


async def check_file_access(user, db, path: str, write: bool = False):
    if is_admin(user):
        return
    acl = await get_acl_for_user(db, user["username"])
    if not path_allowed(acl, path, write):
        raise HTTPException(403, "Access denied")
