"""
限流配置中心
提供配置驱动的限流规则管理
"""
import json
import os
from typing import Dict, Any
from dataclasses import dataclass, asdict


@dataclass
class RateLimitRule:
    """限流规则配置 - 使用dataclass简化配置管理
    """
    capacity: int = 10           # 桶容量（最大突发流量）
    rate: float = 1.0            # 补充速率 (tokens/second)
    burst: int = 5               # 突发容量
    window: int = 3600           # 统计窗口 (秒)
    warmup: int = 0              # 预热时间 (秒)
    enabled: bool = True         # 是否启用
    
    def to_dict(self) -> Dict[str, Any]:
        """转换为字典，用于JSON序列化"""
        return asdict(self)
        
    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> 'RateLimitRule':
        """从字典创建实例，用于JSON反序列化"""
        return cls(**data)


class RateLimitConfig:
    """限流配置管理器 - 实现配置驱动设计
    """
    
    def __init__(self, config_path: str = None):
        """
        初始化配置管理器 - 配置系统的入口点
        """
        self.config_path = config_path or os.path.join(
            os.path.dirname(__file__),   # 当前Python文件所在目录
            'rate_limits.json'           # 默认配置文件名
        )
        
        self.rules: Dict[str, RateLimitRule] = {}
        
        self._load_config()
    
    def _load_config(self):
        """加载配置文件 - 实现智能配置加载"""
        # 默认限流规则：基于业务场景的合理默认值
        default_rules = {
            "/api/v1/auth/send_code": RateLimitRule(
                capacity=10,      # 10次验证码发送
                rate=1.0,         # 每1秒补充1个
                burst=5,          # 允许突发5个
                window=3600,      # 1小时统计窗口
                warmup=0
            ),
            "/api/v1/auth/login": RateLimitRule(
                capacity=5,       # 5次登录尝试
                rate=0.5,         # 每2秒补充1个（更严格）
                burst=2,          # 允许突发2个
                window=300,       # 5分钟统计窗口
                warmup=0
            ),
            "/api/v1/auth/register": RateLimitRule(
                capacity=3,       # 3次注册尝试
                rate=0.2,         # 每5秒补充1个（最严格）
                burst=1,          # 允许突发1个
                window=3600,      # 1小时统计窗口
                warmup=0
            ),
            "/api/v1/auth/verify_code": RateLimitRule(
                capacity=20,      # 20次验证码验证
                rate=2.0,         # 每0.5秒补充1个
                burst=10,         # 允许突发10个
                window=300,       # 5分钟统计窗口
                warmup=0
            ),
            "/api/v1/auth/refresh_token": RateLimitRule(
                capacity=50,      # 50次token刷新
                rate=5.0,         # 每0.2秒补充1个
                burst=20,         # 允许突发20个
                window=3600,      # 1小时统计窗口
                warmup=0
            ),
            "/api/v1/auth/logout": RateLimitRule(
                capacity=100,     # 100次登出
                rate=10.0,        # 每0.1秒补充1个
                burst=50,         # 允许突发50个
                window=3600,      # 1小时统计窗口
                warmup=0
            )
        }
        
        try:
            if os.path.exists(self.config_path):
                # 从配置文件加载
                with open(self.config_path, 'r', encoding='utf-8') as f:
                    config_data = json.load(f)
                    
                # 验证配置格式
                for path, rule_data in config_data.items():
                    try:
                        self.rules[path] = RateLimitRule.from_dict(rule_data)
                    except TypeError as e:
                        print(f"配置格式错误 - 路径 {path}: {e}")
                        # 使用默认值
                        self.rules[path] = default_rules.get(path, RateLimitRule())
                        
            else:
                # 首次使用：创建默认配置
                self.rules = default_rules
                self._save_config()
                print(f"✅ 创建默认配置文件: {self.config_path}")
                
        except json.JSONDecodeError as e:
            print(f"❌ 配置文件格式错误: {e}")
            self.rules = default_rules
        except Exception as e:
            print(f"❌ 加载配置失败: {e}")
            self.rules = default_rules
    
    def _save_config(self):
        """保存配置到文件 - 原子性写入"""
        try:
            # 临时文件写入，保证原子性
            temp_path = f"{self.config_path}.tmp"
            config_data = {
                path: rule.to_dict() 
                for path, rule in self.rules.items()
            }
            
            with open(temp_path, 'w', encoding='utf-8') as f:
                json.dump(config_data, f, indent=2, ensure_ascii=False)
            
            # 原子性替换
            os.rename(temp_path, self.config_path)
            
        except Exception as e:
            print(f"❌ 保存配置失败: {e}")
            # 清理临时文件
            if os.path.exists(temp_path):
                os.remove(temp_path)
    
    def get_rule(self, path: str) -> RateLimitRule:
        """获取指定路径的限流规则"""
        return self.rules.get(path, RateLimitRule())
    
    def update_rule(self, path: str, rule: RateLimitRule):
        """更新限流规则"""
        self.rules[path] = rule
        self._save_config()
    
    def add_rule(self, path: str, rule: RateLimitRule):
        """添加新的限流规则"""
        self.update_rule(path, rule)
    
    def remove_rule(self, path: str):
        """移除限流规则"""
        if path in self.rules:
            del self.rules[path]
            self._save_config()
    
    def get_all_rules(self) -> Dict[str, RateLimitRule]:
        """获取所有限流规则"""
        return self.rules.copy()
    
    def reload_config(self):
        """重新加载配置"""
        self._load_config()


# 全局配置实例
rate_limit_config = RateLimitConfig()