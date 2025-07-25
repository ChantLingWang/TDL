try:
    from pydantic_settings import BaseSettings
except ImportError:
    from pydantic import BaseSettings
from typing import Optional
from .secret_key import get_secret_key

class Settings(BaseSettings):
    """应用配置"""
    # 应用基本信息
    app_name: str = "Auth Service"
    service_id: str = "auth_service"
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
    
    # 邮箱配置
    smtp_server: str = "smtp.qq.com"
    smtp_port: int = 587
    smtp_username: str = "809595872@qq.com"
    smtp_password: str = "flvvqalxrcmlbdhj"
    
    # Redis配置
    redis_host: str = "localhost"
    redis_port: int = 6379
    redis_db: int = 0
    
    class Config:
        env_file = ".env"
        case_sensitive = False

settings = Settings() 