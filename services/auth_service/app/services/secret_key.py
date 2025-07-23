import hashlib
import base64
from datetime import datetime

def get_secret_key():
    """
    获取密钥 - 使用固定元素生成复杂密钥
    """
    name = "LianLingHao"
    fixed_number = 1001
    fixed_field = "liberation"
    
    # 使用SHA256哈希生成密钥，确保每次调用都返回相同的密钥
    combined_string = f"{name}_{fixed_number}_{fixed_field}"
    hash_object = hashlib.sha256(combined_string.encode('utf-8'))
    secret_key = hash_object.hexdigest()
    
    return secret_key
