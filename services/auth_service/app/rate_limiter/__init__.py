"""
分布式限流器包
基于Redis的令牌桶算法实现
"""

from .token_bucket import TokenBucketRateLimiter
from .config_center import RateLimitConfig, rate_limit_config
from .middleware import (
    RateLimitMiddleware, 
    RateLimitDecorator,
    create_rate_limit_middleware,
    ip_identifier,
    user_identifier,
    api_key_identifier
)

__version__ = "1.0.0"
__all__ = [
    "TokenBucketRateLimiter",
    "RateLimitConfig", 
    "rate_limit_config",
    "RateLimitMiddleware",
    "RateLimitDecorator",
    "create_rate_limit_middleware",
    "ip_identifier",
    "user_identifier", 
    "api_key_identifier"
]

# 便捷导入
RateLimiter = TokenBucketRateLimiter
