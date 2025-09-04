"""
FastAPI限流中间件
提供请求级别的限流功能
"""
from typing import Callable, Optional, Dict, Any
from fastapi import FastAPI, Request, HTTPException
from fastapi.responses import JSONResponse
import logging
import time

# 使用绝对导入
from token_bucket import TokenBucketRateLimiter
from config_center import rate_limit_config

logger = logging.getLogger(__name__)


class RateLimitMiddleware:
    """FastAPI限流中间件"""
    
    def __init__(
        self, 
        app: FastAPI, 
        redis_client,
        default_capacity: int = 100,
        default_rate: float = 10.0
    ):
        self.app = app
        self.limiter = TokenBucketRateLimiter(redis_client)
        self.default_capacity = default_capacity
        self.default_rate = default_rate
        
    def get_client_identifier(self, request: Request) -> str:
        """获取客户端标识符"""
        # 优先使用API Key
        api_key = request.headers.get('X-API-Key')
        if api_key:
            return f"api_key:{api_key}"
        
        # 其次使用用户ID
        user_id = getattr(request.state, 'user_id', None)
        if user_id:
            return f"user:{user_id}"
        
        # 最后使用IP地址
        forwarded_for = request.headers.get('X-Forwarded-For')
        if forwarded_for:
            client_ip = forwarded_for.split(',')[0].strip()
        else:
            client_ip = request.client.host
        
        return f"ip:{client_ip}"
    
    async def __call__(self, request: Request, call_next):
        """中间件调用"""
        # 获取限流规则
        path = request.url.path
        rule = rate_limit_config.get_rule(path)
        
        if not rule.enabled:
            # 如果未启用限流，直接通过
            response = await call_next(request)
            return response
        
        # 获取客户端标识符
        identifier = self.get_client_identifier(request)
        limit_key = f"rate_limit:{path}:{identifier}"
        
        # 尝试获取令牌
        success, remaining, wait_time = self.limiter.acquire_token(
            limit_key, rule.capacity, rule.rate
        )
        
        if not success:
            # 限流触发
            logger.warning(
                f"限流触发: {path} - {identifier}, "
                f"剩余: {remaining}, 需等待: {wait_time:.2f}s"
            )
            
            return JSONResponse(
                status_code=429,
                content={
                    "error": "Too Many Requests",
                    "message": f"请求过于频繁，请等待 {wait_time:.2f} 秒",
                    "retry_after": wait_time,
                    "path": path,
                    "identifier": identifier
                },
                headers={
                    "Retry-After": str(int(wait_time)),
                    "X-RateLimit-Remaining": str(remaining),
                    "X-RateLimit-Limit": str(rule.capacity)
                }
            )
        
        # 添加响应头
        response = await call_next(request)
        response.headers["X-RateLimit-Remaining"] = str(remaining)
        response.headers["X-RateLimit-Limit"] = str(rule.capacity)
        
        return response


def rate_limit(
    path: str = None,
    capacity: int = None,
    rate: float = None,
    identifier_type: str = "auto"
):
    """
    限流装饰器
    
    Args:
        path: 限流路径，默认为请求路径
        capacity: 桶容量
        rate: 令牌补充速率
        identifier_type: 标识符类型 ("ip", "user", "api_key", "auto")
    """
    def decorator(func):
        async def wrapper(request: Request, *args, **kwargs):
            # 获取Redis客户端（需要从应用中获取）
            redis_client = getattr(request.app.state, 'redis_client', None)
            if not redis_client:
                return await func(request, *args, **kwargs)
            
            limiter = TokenBucketRateLimiter(redis_client)
            
            # 获取路径
            limit_path = path or request.url.path
            
            # 获取标识符
            if identifier_type == "ip":
                identifier = f"ip:{request.client.host}"
            elif identifier_type == "user":
                user_id = getattr(request.state, 'user_id', 'anonymous')
                identifier = f"user:{user_id}"
            elif identifier_type == "api_key":
                api_key = request.headers.get('X-API-Key', 'anonymous')
                identifier = f"api_key:{api_key}"
            else:  # auto
                # 使用中间件的逻辑
                forwarded_for = request.headers.get('X-Forwarded-For')
                if forwarded_for:
                    client_ip = forwarded_for.split(',')[0].strip()
                else:
                    client_ip = request.client.host
                identifier = f"ip:{client_ip}"
            
            limit_key = f"rate_limit:{limit_path}:{identifier}"
            
            # 获取限流规则
            rule = rate_limit_config.get_rule(limit_path)
            use_capacity = capacity or rule.capacity
            use_rate = rate or rule.rate
            
            # 尝试获取令牌
            success, remaining, wait_time = limiter.acquire_token(
                limit_key, use_capacity, use_rate
            )
            
            if not success:
                return JSONResponse(
                    status_code=429,
                    content={
                        "error": "Too Many Requests",
                        "message": f"请求过于频繁，请等待 {wait_time:.2f} 秒",
                        "retry_after": wait_time
                    },
                    headers={
                        "Retry-After": str(int(wait_time)),
                        "X-RateLimit-Remaining": str(remaining)
                    }
                )
            
            # 添加响应头
            response = await func(request, *args, **kwargs)
            if hasattr(response, 'headers'):
                response.headers["X-RateLimit-Remaining"] = str(remaining)
            
            return response
        
        return wrapper
    return decorator


# 便捷函数
def setup_rate_limiter(app: FastAPI, redis_client):
    """设置限流器"""
    middleware = RateLimitMiddleware(app, redis_client)
    app.add_middleware(type(middleware), dispatch=middleware)
    
    # 存储Redis客户端
    app.state.redis_client = redis_client
    
    logger.info("✅ 限流器已启用")