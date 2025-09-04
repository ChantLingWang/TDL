import redis
import json
import time
import logging
from typing import Tuple, Dict, Any

# 使用绝对导入
from config_center import rate_limit_config

logger = logging.getLogger(__name__)


class TokenBucketRateLimiter:
    """基于Redis的分布式令牌桶限流器"""
    
    def __init__(self, redis_client: redis.Redis):
        self.redis = redis_client
        self.scripts = {}
        self._load_scripts()
    
    def _load_scripts(self):
        """加载Lua脚本"""
        try:
            import os
            script_path = os.path.join(os.path.dirname(__file__), 'lua_scripts', 'token_bucket.lua')
            
            with open(script_path, 'r', encoding='utf-8') as f:
                script_content = f.read()
                
            self.scripts['token_bucket'] = self.redis.register_script(script_content)
            logger.info("✅ Lua脚本加载成功")
            
        except Exception as e:
            logger.error(f"❌ 加载Lua脚本失败: {e}")
            raise
    
    def acquire_token(
        self, 
        key: str, 
        capacity: int = None, 
        rate: float = None,
        requested: int = 1
    ) -> Tuple[bool, int, float]:
        """
        获取令牌
        
        Args:
            key: 限流键
            capacity: 桶容量
            rate: 令牌补充速率(令牌/秒)
            requested: 请求的令牌数(默认为1)
            
        Returns:
            (是否成功, 剩余令牌数, 等待时间)
        """
        if capacity is None or rate is None:
            rule = rate_limit_config.get_rule(key)
            capacity = rule.capacity
            rate = rule.rate
        
        try:
            result = self.scripts['token_bucket'](
                keys=[key],
                args=[capacity, rate, int(time.time() * 1000), requested]
            )
            
            success = bool(result[0])
            remaining = int(result[1])
            wait_time = float(result[2]) / 1000.0  # 转换为秒
            
            return success, remaining, wait_time
            
        except Exception as e:
            logger.error(f"❌ 限流器执行失败: {e}")
            # 降级策略：允许请求通过
            return True, 0, 0.0
    
    def get_remaining_tokens(self, key: str) -> int:
        """获取剩余令牌数"""
        try:
            rule = rate_limit_config.get_rule(key)
            current_tokens = self.redis.hget(key, 'tokens')
            
            if current_tokens is None:
                return rule.capacity
            
            # 处理浮点数字符串
            try:
                return max(0, int(float(current_tokens)))
            except (ValueError, TypeError):
                return rule.capacity
            
        except Exception as e:
            logger.error(f"❌ 获取剩余令牌失败: {e}")
            return 0
    
    def reset_bucket(self, key: str, capacity: int = None):
        """重置令牌桶"""
        if capacity is None:
            rule = rate_limit_config.get_rule(key)
            capacity = rule.capacity
        
        try:
            self.redis.hset(key, mapping={
                'tokens': capacity,
                'last_refill_ms': int(time.time() * 1000),
                'capacity': capacity
            })
            logger.info(f"✅ 重置令牌桶: {key}")
            
        except Exception as e:
            logger.error(f"❌ 重置令牌桶失败: {e}")
    
    def get_stats(self, key: str) -> Dict[str, Any]:
        """获取限流统计信息"""
        try:
            data = self.redis.hgetall(key)
            if not data:
                return {}
            
            def safe_int(value, default=0):
                """安全转换整数"""
                if value is None:
                    return default
                try:
                    return int(float(str(value)))
                except (ValueError, TypeError):
                    return default
            
            return {
                'key': key,
                'tokens': safe_int(data.get('tokens')),
                'capacity': safe_int(data.get('capacity')),
                'last_refill_ms': safe_int(data.get('last_refill_ms')),
                'timestamp': int(time.time() * 1000)
            }
            
        except Exception as e:
            logger.error(f"❌ 获取统计信息失败: {e}")
            return {}