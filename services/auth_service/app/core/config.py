try:
    from pydantic_settings import BaseSettings
except ImportError:
    from pydantic import BaseSettings
from typing import Optional
from services.auth_service.app.core.secret_key import get_secret_key

class Settings(BaseSettings):
    """应用配置"""
    # 应用基本信息
    app_name: str = "Auth Service"
    description: str = "用户登录或注册服务"
    version: str = "1.0.0"
    debug: bool = True
    
    # 服务器配置
    host: str = "127.0.0.1"
    port: int = 9030
    
    # 数据库配置
    mongodb_url: str = "mongodb://localhost:27017"
    database_name: str = "TDL_local_test_database"
    
    # JWT配置
    secret_key: str = get_secret_key()
    algorithm: str = "HS256"
    access_token_expire_minutes: int = 120
    
    # CORS配置
    allowed_origins: list = ["*"]       #允许访问API的来源
    allowed_methods: list = ["*"]       #允许访问API的方法
    allowed_headers: list = ["*"]       #允许访问API的请求头
    
    class Config:
        env_file = ".env"
        case_sensitive = False

settings = Settings() 