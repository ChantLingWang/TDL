-- token_bucket.lua
-- 令牌桶限流算法 - 惰性补充策略
-- KEYS[1]: 限流键 (接口路径+IP/用户标识)
-- ARGV[1]: 桶容量 (capacity)
-- ARGV[2]: 补充速率 (tokens/second)
-- ARGV[3]: 当前时间戳 (毫秒)
-- ARGV[4]: 请求令牌数 (通常为1)

local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

-- 获取当前令牌桶状态
local bucket = redis.call('HMGET', key, 'tokens', 'last_refill_ms')
local tokens = tonumber(bucket[1]) or capacity
local last_refill = tonumber(bucket[2]) or now_ms

-- 计算时间差（秒）
local elapsed_seconds = (now_ms - last_refill) / 1000

-- 计算应补充的令牌数
local tokens_to_add = elapsed_seconds * rate
local new_tokens = math.min(capacity, tokens + tokens_to_add)

-- 检查是否有足够令牌
if new_tokens >= requested then
    -- 扣除令牌
    new_tokens = new_tokens - requested
    
    -- 更新令牌桶状态
    redis.call('HMSET', key, 
        'tokens', new_tokens, 
        'last_refill_ms', now_ms
    )
    
    -- 设置过期时间（防止内存泄漏）
    redis.call('EXPIRE', key, 3600)
    
    -- 返回成功，剩余令牌数，等待时间(0)
    return {1, math.floor(new_tokens), 0}
else
    -- 更新令牌数（不扣除，因为请求被拒绝）
    redis.call('HMSET', key, 
        'tokens', new_tokens, 
        'last_refill_ms', now_ms
    )
    redis.call('EXPIRE', key, 3600)
    
    -- 计算需要等待的时间
    local wait_time = math.ceil((requested - new_tokens) / rate)
    
    -- 返回失败，剩余令牌数，等待时间
    return {0, math.floor(new_tokens), wait_time}
end