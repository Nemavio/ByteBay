from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    secret_key: str = "dev-secret-change-me"
    db_path: str = "/data/bytebay.db"
    agent_socket: str = "/run/bytebay/agent.sock"
    engine_socket: str = "/run/bytebay/engine.sock"
    agent_token: str = ""
    engine_token: str = ""
    admin_user: str = "admin"
    admin_password: str = "admin"
    token_expire_minutes: int = 60 * 24

    class Config:
        env_prefix = "BYTEBAY_"
        env_file = ".env"


settings = Settings()
