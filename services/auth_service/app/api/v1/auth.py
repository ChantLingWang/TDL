import asyncio
import secrets
from datetime import datetime
from datetime import timezone
from fastapi import APIRouter, HTTPException
from app.models.auth_model import LoginRequest,SendCodeRequest,VerifyCodeRequest,VerifyCodeLoginRequest,ResetPasswordRequest,RefreshTokenRequest,LogoutRequest
from app.database.mongodb_user_service import MongoDBUserService,db_manager
import logging
from app.database.redis_user_service import RedisUserService
from app.database.mongodb_user_token_service import MongoDBUserTokenService
from app.services.email_service import EmailService
from app.services.jwt_service import JWTUtils
from app.utils.error_code import ErrorCodeEnum
from fastapi import Request
import bcrypt
from app.api.v1.const import Status

logger = logging.getLogger(__name__)

router = APIRouter()

MAX_CODE_ATTEMPTS = 5
CODE_ATTEMPTS_TTL = 600


def _validate_password(password: str) -> None:
    """校验密码强度与长度；bcrypt 只使用前 72 字节，过长会被静默截断。"""
    if len(password) < 8:
        raise HTTPException(status_code=400, detail="密码长度至少 8 位")
    if len(password.encode("utf-8")) > 72:
        raise HTTPException(status_code=400, detail="密码过长（最多 72 字节）")


def _verify_code_or_raise(redis_service, email: str, code: str) -> None:
    """校验邮箱验证码：

    - 恒定时间比较，避免时序侧信道；
    - 同一邮箱最多连续失败 MAX_CODE_ATTEMPTS 次，防止爆破；
    - 校验成功后一次性删除验证码与失败计数。
    """
    attempts_key = f"code_attempts:{email}"
    attempts_raw = redis_service.get_code(attempts_key)
    attempts = int(attempts_raw) if attempts_raw else 0
    if attempts >= MAX_CODE_ATTEMPTS:
        raise HTTPException(status_code=429, detail="验证码错误次数过多，请重新获取验证码")

    stored = redis_service.get_code(email)
    if stored is None:
        raise HTTPException(
            status_code=400,
            detail=ErrorCodeEnum.USER_VERIFICATION_CODE_EXPIRED.message,
        )

    if not secrets.compare_digest(stored, code):
        redis_service.set_code(attempts_key, str(attempts + 1), CODE_ATTEMPTS_TTL)
        raise HTTPException(
            status_code=400,
            detail=ErrorCodeEnum.USER_VERIFICATION_CODE_INCORRECT.message,
        )

    redis_service.delete_code(email)
    redis_service.delete_code(attempts_key)


@router.get("/health")
async def health_check():
    """健康检查端点"""
    return {"status": "ok", "service": "auth_service"}



async def get_user_service():
    """获取用户服务实例并检查数据库连接"""
    is_connected = await db_manager.test_connection()
    if not is_connected:
        raise HTTPException(status_code=ErrorCodeEnum.DATABASE_CONNECTION_ERROR.http_status, detail=ErrorCodeEnum.DATABASE_CONNECTION_ERROR.message)
    return MongoDBUserService(db_manager)

async def get_user_token_service():
    """获取token服务实例并检查数据库连接"""
    redis_service = RedisUserService()
    is_connected = redis_service.test_connection()
    if not is_connected:
        raise HTTPException(status_code=ErrorCodeEnum.REDIS_CONNECTION_ERROR.http_status, detail=ErrorCodeEnum.REDIS_CONNECTION_ERROR.message)
    return MongoDBUserTokenService(db_manager)


@router.post("/send_code",
    summary="发送验证码",
    description="发送验证码接口，发送验证码到用户邮箱",
    response_description="返回发送结果"
)
async def send_code(request:Request,data: SendCodeRequest):
    """发送验证码接口"""
    email_service = EmailService()
    rate_key = f"rate:code:{data.email}"
    redis_rate = RedisUserService()
    # 原子限流：SET NX，并发请求只有一个能成功，避免 check-then-set 竞态
    if not redis_rate.try_set_nx(rate_key, "1", 60):
        raise HTTPException(status_code=429, detail="发送过于频繁，请稍后再试")

    try:
        await asyncio.to_thread(email_service.send_email, data.email)
        return {"message": "验证码发送成功"}
    except Exception as e:
        # 发送失败时释放限流标记，避免误伤下一次正常请求
        redis_rate.delete_code(rate_key)
        # 记录详细的错误日志，便于问题排查
        logger.error(f"发送验证码失败 - 邮箱: {data.email}, 错误类型: {type(e).__name__}, 错误信息: {str(e)}", exc_info=True)
        raise HTTPException(
            status_code=ErrorCodeEnum.EMAIL_SEND_ERROR.http_status, 
            detail=ErrorCodeEnum.EMAIL_SEND_ERROR.message
        )


@router.post("/register",
    summary="验证注册验证码",
    description="验证注册验证码接口，验证注册验证码",
    response_description="返回用户字段和token"
)
async def register(request:Request,data: VerifyCodeRequest):
    """验证注册验证码接口"""
    try:
        _validate_password(data.password)
        redis_client = RedisUserService()

        user_service = await get_user_service()

        # 校验验证码（校验通过后自动删除验证码与失败计数）
        _verify_code_or_raise(redis_client, data.email, data.code)

        user_exist = await user_service.get_user_by_email(data.email)

        if user_exist:
            raise HTTPException(status_code=400, detail=ErrorCodeEnum.USER_ALREADY_EXISTS.message)

        # 获取下一个用户ID
        user_id = await user_service.get_next_user_id()

        # 添加创建时间字段，用于同步给其他微服务
        current_time = datetime.now(timezone.utc).isoformat()
        
	# 先构建核心用户信息（用于token生成和事件发布）
        user = {
            "user_id": user_id,
            "username": data.username,
            "email": data.email,
            "created_at": current_time,
            "status": Status.PENDING, # 初始状态为 pending (准备中/中间态)
        }
        
        # 构建完整用户数据（包含密码和其他字段，用于存放数据库）
        user_data = {
            **user,
            "password": bcrypt.hashpw(data.password.encode('utf-8'),bcrypt.gensalt()).decode('utf-8'),
        }

        # 生成token
        access_token = JWTUtils.create_access_token(user)
        try:
            token_service = MongoDBUserTokenService(db_manager)
            refresh_token = await token_service.create_user_token(user)
        except Exception as e:
            logger.warning(f"refresh_token 创建失败（注册继续）: {e}")
            refresh_token = ""

        await user_service.create_user(user_data)

        return {
            "message": Status.SUCCESS,
            "data": {
                "user": user,
                "access_token": access_token,
                "refresh_token": refresh_token,
            }
        }
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"用户注册失败 - 错误类型: {type(e).__name__}, 错误信息: {str(e)}", exc_info=True)
        raise HTTPException(
            status_code=400, 
            detail=ErrorCodeEnum.USER_REGISTER_ERROR.message
        )


@router.post("/verify_code_login",
    summary="验证登录验证码",
    description="验证登录验证码接口，验证登录验证码",
    response_description="返回用户字段和token"
)
async def verify_code_login(request:Request,data: VerifyCodeLoginRequest):
    """验证登录验证码接口"""
    
    redis_client = RedisUserService()
    
    user_service = await get_user_service()
    user_token_service = await get_user_token_service()

    user_data = await user_service.get_user_by_email(data.email)

    if not user_data:
        raise HTTPException(status_code=ErrorCodeEnum.USER_NOT_FOUND.http_status, detail=ErrorCodeEnum.USER_NOT_FOUND.message)

    # 校验验证码（校验通过后自动删除验证码与失败计数）
    _verify_code_or_raise(redis_client, data.email, data.code)

    access_token = JWTUtils.create_access_token(user_data)
    
    refresh_token = JWTUtils.create_refresh_token(user_data)
    
    await user_token_service.update_user_refresh_token(user_data['user_id'], refresh_token)
    
    return{
        "message": "success",
        "data": {
            "user": user_data,
            "access_token": access_token,
            "refresh_token": refresh_token,
        }
    }


@router.post("/login",
    summary="用户登录",
    description="用户登录接口，验证用户凭据",
    response_description="返回用户字段和token"
)
async def login(request:Request,data: LoginRequest):
    """用户登录接口"""
    
    user_service = await get_user_service()
    
    # 获取包含密码的用户数据用于验证
    user_with_password = await user_service.get_user_by_email_with_password(data.email)
    if not user_with_password:
        raise HTTPException(status_code=ErrorCodeEnum.USER_NOT_FOUND.http_status, detail=ErrorCodeEnum.USER_NOT_FOUND.message)
    
    # 验证密码（密码现在是字符串，需要转换回bytes进行验证）
    if not bcrypt.checkpw(data.password.encode('utf-8'), user_with_password['password'].encode('utf-8')):
        raise HTTPException(status_code=ErrorCodeEnum.USER_PASSWORD_INCORRECT.http_status, detail=ErrorCodeEnum.USER_PASSWORD_INCORRECT.message)
    
    # 获取不含密码的用户数据用于JWT和返回
    user = await user_service.get_user_by_email(data.email)
    
    # 生成token
    access_token = JWTUtils.create_access_token(user)
    refresh_token = JWTUtils.create_refresh_token(user)
    
    # 持久化refresh_token
    try:
        user_token_service = await get_user_token_service()
        await user_token_service.update_user_refresh_token(user["user_id"], refresh_token)
    except HTTPException:
        raise
    except Exception as e:
        logger.warning(f"refresh_token 持久化失败（登录继续）: {e}")

    
    return {
        "message": "success",
        "data": {
            "user": user,
            "access_token": access_token,
            "refresh_token": refresh_token
        }
    }


@router.post("/reset_password",
    summary="重置密码",
    description="重置密码接口，重置密码",
    response_description="返回重置结果"
)
async def reset_password(request:Request,data: ResetPasswordRequest):
    # 前端传的 user_id 是 string，MongoDB 里是 int，统一转 int
    try:
        user_id = int(data.user_id)
    except ValueError:
        user_id = data.user_id

    _validate_password(data.password)

    redis_service = RedisUserService()
    user_service = await get_user_service()

    user_data = await user_service.get_user_by_email(data.email,
        {
            "_id": 0,
            "user_id": 1, 
            "username": 1, 
            "email": 1, 
            "password": 1
        }
    )
    if not user_data:
        raise HTTPException(status_code=ErrorCodeEnum.USER_NOT_FOUND.http_status, detail=ErrorCodeEnum.USER_NOT_FOUND.message)
    
    # 检查用户id是否匹配,防止串改id或邮箱，保证安全
    if user_data['user_id'] != user_id:
        raise HTTPException(status_code=ErrorCodeEnum.USER_ID_INCORRECT.http_status, detail=ErrorCodeEnum.USER_ID_INCORRECT.message)

    # 校验验证码（校验通过后自动删除验证码与失败计数）
    _verify_code_or_raise(redis_service, data.email, data.code)

    # 检查新密码是否与旧密码相同
    if bcrypt.checkpw(data.password.encode('utf-8'), user_data['password'].encode('utf-8')):
        raise HTTPException(status_code=ErrorCodeEnum.USER_PASSWORD_SAME.http_status, detail=ErrorCodeEnum.USER_PASSWORD_SAME.message)

    #对新密码进行哈希加密
    new_password = bcrypt.hashpw(data.password.encode('utf-8'),bcrypt.gensalt()).decode('utf-8')

    #更新数据库中的密码
    result = await user_service.update_user_password_by_id(user_id,new_password)

    if result.modified_count == 0:
        raise HTTPException(status_code=ErrorCodeEnum.USER_PASSWORD_RESET_FAILED.http_status, detail=ErrorCodeEnum.USER_PASSWORD_RESET_FAILED.message)

    # 改密后吊销旧 refresh token，防止旧会话继续使用
    token_service = MongoDBUserTokenService(db_manager)
    await token_service.update_user_token_is_valid(user_id, False)

    return {"message": "success"}


@router.post("/refresh_token",
summary="刷新access_token",
response_description="返回刷新后的access_token"
)
async def refresh_token(request:Request,data: RefreshTokenRequest):
    # 前端传的 user_id 是 string，MongoDB 里是 int，统一转 int
    try:
        user_id = int(data.user_id)
    except ValueError:
                user_id = data.user_id
    user_service = await get_user_service()
    user_token_service = await get_user_token_service()
    
    # 从token数据库中查询用户信息
    user_token_data = await user_token_service.get_user_token_by_user_id(user_id,
    {
        "_id": 0,
        "user_id": 1, 
        "email": 1, 
        "refresh_token": 1,
        "is_valid": 1,
    })
    
    #检查token是否存在
    if not user_token_data:
        raise HTTPException(status_code=ErrorCodeEnum.USER_REFRESH_TOKEN_INCORRECT.http_status, detail=ErrorCodeEnum.USER_REFRESH_TOKEN_INCORRECT.message)
    
    #取出token
    payload = JWTUtils.verify_token(data.refresh_token)
    
    #检查token是否有效
    if payload.get('status') == 'error':
        raise HTTPException(status_code=ErrorCodeEnum.USER_REFRESH_TOKEN_INCORRECT.http_status, detail=ErrorCodeEnum.USER_REFRESH_TOKEN_INCORRECT.message)
    
    if user_token_data.get('is_valid') == False:
        raise HTTPException(status_code=ErrorCodeEnum.USER_REFRESH_TOKEN_INCORRECT.http_status, detail=ErrorCodeEnum.USER_REFRESH_TOKEN_INCORRECT.message)
    
    token_payload = payload.get('payload') or {}

    # 必须是 refresh 类型 token，防止用 access token 换取新 token
    if token_payload.get('type') != 'refresh':
        raise HTTPException(status_code=ErrorCodeEnum.USER_REFRESH_TOKEN_INCORRECT.http_status, detail=ErrorCodeEnum.USER_REFRESH_TOKEN_INCORRECT.message)

    # 请求携带的 token 必须与数据库中存储的当前 token 一致
    stored_token = user_token_data.get('refresh_token')
    if not stored_token or not secrets.compare_digest(data.refresh_token, stored_token):
        raise HTTPException(status_code=ErrorCodeEnum.USER_REFRESH_TOKEN_INCORRECT.http_status, detail=ErrorCodeEnum.USER_REFRESH_TOKEN_INCORRECT.message)
    
    #检查token中的用户id是否与请求中的用户id一致
    if token_payload.get('user_id') != user_id:
        raise HTTPException(status_code=ErrorCodeEnum.USER_REFRESH_TOKEN_INCORRECT.http_status, detail=ErrorCodeEnum.USER_REFRESH_TOKEN_INCORRECT.message)
    
    #从用户数据库中获取用户
    user_id_data = await user_service.get_user_by_id(user_id,
    {
        "_id": 0, "user_id": 1,
    })
    
    #检查用户是否存在
    if not user_id_data:
        raise HTTPException(status_code=ErrorCodeEnum.USER_NOT_FOUND.http_status, detail=ErrorCodeEnum.USER_NOT_FOUND.message)
    
    # 生成新的token
    new_access_token = JWTUtils.create_access_token(user_id_data)
    new_refresh_token = JWTUtils.create_refresh_token(user_id_data)
    
    # 更新用户token
    await user_token_service.update_user_refresh_token(user_id, new_refresh_token)
    
    return {
        "message": "success",
        "data": {
            "user": user_id_data,
            "access_token": new_access_token, 
            "refresh_token": new_refresh_token,
        }
    }
        
         
@router.post("/logout",
summary="退出登录",
description="退出登录接口，退出登录",
response_description="返回退出登录结果"
)
async def logout(request:Request,data: LogoutRequest):
    # 前端传的 user_id 是 string，MongoDB 里是 int，统一转 int
    try:
        user_id = int(data.user_id)
    except ValueError:
        user_id = data.user_id
    user_service = await get_user_service()
    user_token_service = await get_user_token_service()
    
    # 验证用户是否存在
    user_data = await user_service.get_user_by_id(user_id,
    {
        "_id": 0, "user_id": 1, 
    })
    
    if not user_data:
        # 用户不存在或被删除，直接返回成功
        return {
            "message": "success",
        }

    # 校验请求携带的 refresh token 与库中一致，防止仅凭 user_id 登出他人
    token_data = await user_token_service.get_user_token_by_user_id(user_id, {
        "_id": 0,
        "refresh_token": 1,
        "is_valid": 1,
    })
    stored_token = token_data.get("refresh_token") if token_data else ""
    if stored_token and not secrets.compare_digest(data.refresh_token, stored_token):
        raise HTTPException(
            status_code=ErrorCodeEnum.USER_REFRESH_TOKEN_INCORRECT.http_status,
            detail=ErrorCodeEnum.USER_REFRESH_TOKEN_INCORRECT.message,
        )

    # 撤销用户的refresh token，无论是否成功都返回成功
    await user_token_service.update_user_token_is_valid(user_id, False)

    return {
        "message": "success",
    }
