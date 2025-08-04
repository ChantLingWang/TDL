from pydantic_settings import BaseSettings
from services.auth_service.app.core.secret_key import get_secret_key

class Settings(BaseSettings):
    """应用配置"""
    # 应用基本信息
    app_name: str = "User Service"
    service_id: str = "user_service"
    description: str = "用户信息处理服务"
    version: str = "1.0.0"
    debug: bool = True
    
    # 服务器配置
    host: str = "0.0.0.0"
    port: int = 9040
    
    # Consul配置
    consul_service_name: str = "user_service"
    consul_service_id: str = "user_service"
    consul_service_address: str = "127.0.0.1"
    consul_service_port: int = 9030
    
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
    
    # Redis配置
    redis_host: str = "localhost"
    redis_port: int = 6379
    redis_db: int = 0
    
    #grpc配置
    grpc_host: str = "0.0.0.0"
    grpc_port: int = 50051
    
    class Config:
        env_file = ".env"
        case_sensitive = False

settings = Settings() 