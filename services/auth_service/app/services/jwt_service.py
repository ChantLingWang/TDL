import jwt
import time
import uuid
from typing import Dict,Any,Optional
from ..core.secret_key import get_secret_key


class JWTUtils:
    #配置项
    SECRET_KEY = get_secret_key()
    ALGORITHM = "HS256"
    ACCESS_TOKEN_EXPIRE_MINUTES = 120
    REFRESH_TOKEN_EXPIRE_MINUTES = 60 * 24 * 30 # 30天
    
    
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
            "jti": str(uuid.uuid4()),
            "type": "access"
        }
        
        # 合并传入的用户payload
        payload.update(user_payload)
        
        # 生成token
        access_token = jwt.encode(
            payload=payload,
            key=cls.SECRET_KEY,
            algorithm=cls.ALGORITHM
        )
        return access_token
    
    
    @classmethod
    def create_refresh_token(
        cls,
        user_payload: Dict[str, Any],
        expire_minutes: int = None
    ) -> str:
        # 如果过期时间未提供，则使用默认值
        if expire_minutes is None:
            expire_minutes = cls.REFRESH_TOKEN_EXPIRE_MINUTES
        
        # 基础的payload信息传递，过期时间，创建时间，唯一标识
        payload = {
            "exp": time.time() + expire_minutes * 60,
            "iat": time.time(),
            "jti": str(uuid.uuid4()),
            "type": "refresh"
        }
        
        # 合并传入的用户payload
        payload.update(user_payload)
        
        # 生成token
        refresh_token = jwt.encode(
            payload=payload,
            key=cls.SECRET_KEY,
            algorithm=cls.ALGORITHM
        )
        return refresh_token
    
    
    @classmethod
    def verify_token(cls, token: str) -> Optional[Dict[str, Any]]:
        try:
            # 解码token，验证签名和有效期
            payload = jwt.decode(token, cls.SECRET_KEY, algorithms=[cls.ALGORITHM]) #使用jwt库的decode方法解码token，SECRET_KEY：密钥，ALGORITHM：算法
            return {"status": "success", "payload": payload}  # 返回有效的payload
        except jwt.ExpiredSignatureError:
            # 处理过期的token
            return {"status": "error", "message": "Token已过期"}
        except jwt.InvalidTokenError:
            # 处理无效的token
            return {"status": "error", "message": "无效的Token"}
