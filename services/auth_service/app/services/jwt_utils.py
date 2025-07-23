import jwt
import time
import uuid
from typing import Dict,Any,Optional
from .secret_key import get_secret_key


class JWTUtils:
    #配置项
    SECRET_KEY = get_secret_key()
    ALGORITHM = "HS256"
    ACCESS_TOKEN_EXPIRE_MINUTES = 120
    
    @classmethod
    def create_access_token(
        cls,
        user_payload: Dict[str, Any],
        expire_minutes: int = None
    ) -> str:
        # 如果过期时间未提供，则使用默认值
        if expire_minutes is None:
            expire_minutes = cls.ACCESS_TOKEN_EXPIRE_MINUTES
        
        # 基础的payload信息传递，过期时间，创建时间，唯一标识
        payload = {
            "exp": time.time() + expire_minutes * 60,
            "iat": time.time(),
            "jti": str(uuid.uuid4())
        }
        
        # 合并传入的用户payload
        payload.update(user_payload)
        
        # 生成token
        token = jwt.encode(
            payload=payload,
            key=cls.SECRET_KEY,
            algorithm=cls.ALGORITHM
        )
        return token
    
    