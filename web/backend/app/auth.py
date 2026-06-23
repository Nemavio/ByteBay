from datetime import datetime, timedelta, timezone

from fastapi import Depends, HTTPException, status
from fastapi.security import OAuth2PasswordBearer
from jose import JWTError, jwt
from passlib.context import CryptContext

from app.config import settings
from app.database import get_db, get_user_by_username

pwd_context = CryptContext(schemes=["bcrypt"], deprecated="auto")
oauth2_scheme = OAuth2PasswordBearer(tokenUrl="/api/v1/auth/login")

ALGORITHM = "HS256"


def verify_password(plain: str, hashed: str) -> bool:
    return pwd_context.verify(plain, hashed)


def create_access_token(username: str) -> str:
    expire = datetime.now(timezone.utc) + timedelta(minutes=settings.token_expire_minutes)
    return jwt.encode({"sub": username, "exp": expire}, settings.secret_key, algorithm=ALGORITHM)


async def get_current_user(token: str = Depends(oauth2_scheme), db=Depends(get_db)):
    cred_exc = HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Invalid credentials")
    try:
        payload = jwt.decode(token, settings.secret_key, algorithms=[ALGORITHM])
        username = payload.get("sub")
        if not username:
            raise cred_exc
    except JWTError:
        raise cred_exc
    user = await get_user_by_username(db, username)
    if not user:
        raise cred_exc
    return user
