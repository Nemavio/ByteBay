import aiosqlite
from passlib.context import CryptContext

from app.config import settings

pwd_context = CryptContext(schemes=["bcrypt"], deprecated="auto")

SCHEMA = """
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    web_role TEXT NOT NULL DEFAULT 'admin',
    samba_enabled INTEGER NOT NULL DEFAULT 0,
    ftp_enabled INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE IF NOT EXISTS folder_acl (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL,
    username TEXT NOT NULL,
    can_read INTEGER NOT NULL DEFAULT 1,
    can_write INTEGER NOT NULL DEFAULT 0,
    UNIQUE(path, username)
);
"""


async def get_db():
    db = await aiosqlite.connect(settings.db_path)
    db.row_factory = aiosqlite.Row
    try:
        yield db
    finally:
        await db.close()


async def _column_exists(db: aiosqlite.Connection, table: str, column: str) -> bool:
    cur = await db.execute(f"PRAGMA table_info({table})")
    cols = {row[1] for row in await cur.fetchall()}
    return column in cols


async def _migrate(db: aiosqlite.Connection):
    if not await _column_exists(db, "users", "id"):
        return
    if await _column_exists(db, "users", "is_admin") and not await _column_exists(db, "users", "web_role"):
        await db.execute("ALTER TABLE users ADD COLUMN web_role TEXT NOT NULL DEFAULT 'admin'")
        await db.execute("UPDATE users SET web_role = CASE WHEN is_admin = 1 THEN 'admin' ELSE 'viewer' END")
    if not await _column_exists(db, "users", "web_role"):
        await db.execute("ALTER TABLE users ADD COLUMN web_role TEXT NOT NULL DEFAULT 'admin'")
    if not await _column_exists(db, "users", "samba_enabled"):
        await db.execute("ALTER TABLE users ADD COLUMN samba_enabled INTEGER NOT NULL DEFAULT 0")
    if not await _column_exists(db, "users", "ftp_enabled"):
        await db.execute("ALTER TABLE users ADD COLUMN ftp_enabled INTEGER NOT NULL DEFAULT 0")
    await db.execute(
        "CREATE TABLE IF NOT EXISTS folder_acl ("
        "id INTEGER PRIMARY KEY AUTOINCREMENT, path TEXT NOT NULL, username TEXT NOT NULL,"
        "can_read INTEGER NOT NULL DEFAULT 1, can_write INTEGER NOT NULL DEFAULT 0,"
        "UNIQUE(path, username))"
    )
    await db.commit()


async def init_db():
    async with aiosqlite.connect(settings.db_path) as db:
        await db.executescript(SCHEMA)
        await _migrate(db)
        cur = await db.execute("SELECT COUNT(*) FROM users")
        row = await cur.fetchone()
        if row[0] == 0:
            await db.execute(
                "INSERT INTO users (username, password_hash, web_role, samba_enabled, ftp_enabled) "
                "VALUES (?, ?, 'admin', 1, 1)",
                (settings.admin_user, pwd_context.hash(settings.admin_password)),
            )
            await db.execute(
                "INSERT INTO folder_acl (path, username, can_read, can_write) VALUES ('/volumes', ?, 1, 1)",
                (settings.admin_user,),
            )
            await db.execute(
                "INSERT INTO folder_acl (path, username, can_read, can_write) VALUES ('/data', ?, 1, 1)",
                (settings.admin_user,),
            )
            await db.commit()


def user_row_dict(row) -> dict:
    if not row:
        return {}
    d = dict(row)
    d["is_admin"] = d.get("web_role") == "admin"
    return d


async def get_user_by_username(db: aiosqlite.Connection, username: str):
    cur = await db.execute("SELECT * FROM users WHERE username = ?", (username,))
    return await cur.fetchone()


async def list_users(db: aiosqlite.Connection):
    cur = await db.execute(
        "SELECT id, username, web_role, samba_enabled, ftp_enabled, created_at FROM users ORDER BY id"
    )
    return await cur.fetchall()


async def create_user(
    db: aiosqlite.Connection,
    username: str,
    password: str,
    web_role: str = "viewer",
    samba: bool = False,
    ftp: bool = False,
):
    await db.execute(
        "INSERT INTO users (username, password_hash, web_role, samba_enabled, ftp_enabled) "
        "VALUES (?, ?, ?, ?, ?)",
        (username, pwd_context.hash(password), web_role, int(samba), int(ftp)),
    )
    await db.commit()


async def update_user(db: aiosqlite.Connection, user_id: int, **fields):
    allowed = {"web_role", "samba_enabled", "ftp_enabled", "password_hash"}
    sets = []
    vals = []
    for k, v in fields.items():
        if k not in allowed:
            continue
        sets.append(f"{k} = ?")
        vals.append(v)
    if not sets:
        return
    vals.append(user_id)
    await db.execute(f"UPDATE users SET {', '.join(sets)} WHERE id = ?", vals)
    await db.commit()


async def delete_user(db: aiosqlite.Connection, user_id: int):
    cur = await db.execute("SELECT username FROM users WHERE id = ?", (user_id,))
    row = await cur.fetchone()
    if row:
        await db.execute("DELETE FROM folder_acl WHERE username = ?", (row[0],))
    await db.execute("DELETE FROM users WHERE id = ?", (user_id,))
    await db.commit()


async def list_acl(db: aiosqlite.Connection):
    cur = await db.execute("SELECT id, path, username, can_read, can_write FROM folder_acl ORDER BY path, username")
    return await cur.fetchall()


async def set_acl(db: aiosqlite.Connection, path: str, username: str, can_read: bool, can_write: bool):
    await db.execute(
        "INSERT INTO folder_acl (path, username, can_read, can_write) VALUES (?, ?, ?, ?) "
        "ON CONFLICT(path, username) DO UPDATE SET can_read=excluded.can_read, can_write=excluded.can_write",
        (path, username, int(can_read), int(can_write)),
    )
    await db.commit()


async def delete_acl(db: aiosqlite.Connection, acl_id: int):
    await db.execute("DELETE FROM folder_acl WHERE id = ?", (acl_id,))
    await db.commit()


async def get_acl_for_user(db: aiosqlite.Connection, username: str):
    cur = await db.execute(
        "SELECT path, can_read, can_write FROM folder_acl WHERE username = ?", (username,)
    )
    return await cur.fetchall()


async def users_for_engine(db: aiosqlite.Connection):
    cur = await db.execute(
        "SELECT username, password_hash, web_role, samba_enabled, ftp_enabled FROM users"
    )
    return await cur.fetchall()
