import asyncio
from email import message
from fastapi import APIRouter, HTTPException, Depends
from services.auth_service.app.models.auth_model import LoginRequest,SendCodeRequest,VerifyCodeRequest,VerifyCodeLoginRequest,ResetPasswordRequest,RefreshTokenRequest,LogoutRequest
from services.auth_service.app.database.mongodb_user_service import MongoDBUserService,db_manager
from services.auth_service.app.database.redis_user_service import RedisUserService
from services.auth_service.app.database.mongodb_user_token_service import MongoDBUserTokenService
from services.auth_service.app.services.email_service import EmailService
from services.auth_service.app.services.jwt_service import JWTUtils
from services.auth_service.app.utils.error_code import ErrorCodeEnum
from fastapi import Request
import bcrypt


router = APIRouter()


async def get_user_service():
    """获取用户服务实例并检查数据库连接"""
    is_connected = await db_manager.test_connection()
    if not is_connected:
        raise HTTPException(status_code=ErrorCodeEnum.DATABASE_CONNECTION_ERROR.code, detail=ErrorCodeEnum.DATABASE_CONNECTION_ERROR.message)
    return MongoDBUserService(db_manager)


@router.post("/send_code",
    summary="发送验证码",
    description="发送验证码接口，发送验证码到用户邮箱",
    response_description="返回发送结果"
)
async def send_code(request:Request,data: SendCodeRequest):
    """发送验证码接口"""
    email_service = EmailService()
    try:
        email_service.send_email(data.email)
        
        return{
            "message": "验证码发送成功",
            }
    except Exception as e:
        raise HTTPException(status_code=ErrorCodeEnum.EMAIL_SEND_ERROR.code, detail=ErrorCodeEnum.EMAIL_SEND_ERROR.message)


@router.post("/register",
    summary="验证注册验证码",
    description="验证注册验证码接口，验证注册验证码",
    response_description="返回用户字段和token"
)
async def register(request:Request,data: VerifyCodeRequest):
    """验证注册验证码接口"""
    try:
        redis_client = RedisUserService()
        
        user_service = await get_user_service()
        
        code = redis_client.get_code(data.email)    #redis返回的是bytes类型，需要在下面处理为字符串类型才能比较，否则无法比较
        
        # 检查验证码是否存在
        if code is None:
            raise HTTPException(status_code=ErrorCodeEnum.USER_VERIFICATION_CODE_EXPIRED.code, detail=ErrorCodeEnum.USER_VERIFICATION_CODE_EXPIRED.message)
        
        # 将bytes类型转换为字符串进行比较
        code_str = code.decode('utf-8') if isinstance(code, bytes) else str(code)
        
        redis_client.delete_code(data.email)
        
        user = await user_service.get_user_by_email(data.email)
        
        if user:
            raise HTTPException(status_code=ErrorCodeEnum.USER_ALREADY_EXISTS.code, detail=ErrorCodeEnum.USER_ALREADY_EXISTS.message)
        
        if code_str != data.code:
            raise HTTPException(status_code=ErrorCodeEnum.USER_VERIFICATION_CODE_INCORRECT.code, detail=ErrorCodeEnum.USER_VERIFICATION_CODE_INCORRECT.message)
        
        # 创建新用户
        user_id = await user_service.get_next_user_id()
        user_data = {
            "user_id": user_id,
            "username": data.username,
            "email": data.email,
            "password": bcrypt.hashpw(data.password.encode('utf-8'),bcrypt.gensalt()).decode('utf-8'),
        }
        await user_service.create_user(user_data)
        
        # 从数据库中获取完整的用户数据
        user = await user_service.get_user_by_id(user_id,
            {
                "_id": 0,
                "user_id": 1, 
                "username": 1, 
                "email": 1, 
                "create_time": 1,
                "password": 0
            }
        )
        
        #生成token
        access_token = JWTUtils.create_access_token(user)
        refresh_token = await MongoDBUserTokenService.create_user_token(user)
        
        return {
            "message": "success",
            "data": {
                "user": user,
                "access_token": access_token,
                "refresh_token": refresh_token,
            }
        }
        #事务补偿，当用户创建但token创建失败时，或者其他操作失败时，需要回滚用户创建操作
    except Exception as e:
        if user_id:
            await user_service.delete_user_by_id(user_id)
        raise HTTPException(status_code=ErrorCodeEnum.USER_REGISTER_ERROR.code, detail=ErrorCodeEnum.USER_REGISTER_ERROR.message)


@router.post("/verify_code_login",
    summary="验证登录验证码",
    description="验证登录验证码接口，验证登录验证码",
    response_description="返回用户字段和token"
)
async def verify_code_login(request:Request,data: VerifyCodeLoginRequest):
    """验证登录验证码接口"""
    
    redis_client = RedisUserService()
    
    user_service = await get_user_service()
    
    user_data, code = await asyncio.gather(
    user_service.get_user_by_email(data.email),
    redis_client.get_code(data.email)
    )
    
    if not user_data:
        raise HTTPException(status_code=ErrorCodeEnum.USER_NOT_FOUND.code, detail=ErrorCodeEnum.USER_NOT_FOUND.message)
    
    code_str = code.decode('utf-8') if isinstance(code, bytes) else str(code)

    if code_str != data.code:
        raise HTTPException(status_code=ErrorCodeEnum.USER_VERIFICATION_CODE_INCORRECT.code, detail=ErrorCodeEnum.USER_VERIFICATION_CODE_INCORRECT.message)
    else:
        redis_client.delete_code(data.email)

    access_token = JWTUtils.create_access_token(user_data)
    
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
        raise HTTPException(status_code=ErrorCodeEnum.USER_NOT_FOUND.code, detail=ErrorCodeEnum.USER_NOT_FOUND.message)
    
    # 验证密码（密码现在是字符串，需要转换回bytes进行验证）
    if not bcrypt.checkpw(data.password.encode('utf-8'), user_with_password['password'].encode('utf-8')):
        raise HTTPException(status_code=ErrorCodeEnum.USER_PASSWORD_INCORRECT.code, detail=ErrorCodeEnum.USER_PASSWORD_INCORRECT.message)
    
    # 获取不含密码的用户数据用于JWT和返回
    user = await user_service.get_user_by_email(data.email)
    
    # 生成token
    access_token = JWTUtils.create_access_token(user)
    refresh_token = JWTUtils.create_refresh_token(user)
    
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
    """重置密码接口"""
    
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
    
    # 检查用户id是否匹配,防止串改id或邮箱，保证安全
    if user_data['user_id'] != data.user_id:
        raise HTTPException(status_code=ErrorCodeEnum.USER_ID_INCORRECT.code, detail=ErrorCodeEnum.USER_ID_INCORRECT.message)
    
    #检查用户是否存在于数据库中，若不在直接报错给前端
    if not user_data:
        raise HTTPException(status_code=ErrorCodeEnum.USER_NOT_FOUND.code, detail=ErrorCodeEnum.USER_NOT_FOUND.message)
    
    # 从redis中取出验证码
    code = redis_service.get_code(data.email)
    
    # 检查验证码是否过期（验证码有时效性，过期则redis查不到）
    if code is None:
        raise HTTPException(status_code=ErrorCodeEnum.USER_VERIFICATION_CODE_EXPIRED.code, detail=ErrorCodeEnum.USER_VERIFICATION_CODE_EXPIRED.message)
    
    # 对验证码进行格式转换方便对比
    code_str = code.decode('utf-8') if isinstance(code, bytes) else str(code)
    
    #检验验证码是否正确，若不正确直接报错
    if code_str != data.code:
        raise HTTPException(status_code=ErrorCodeEnum.USER_VERIFICATION_CODE_INCORRECT.code, detail=ErrorCodeEnum.USER_VERIFICATION_CODE_INCORRECT.message)
    
    # 检查新密码是否与旧密码相同
    if bcrypt.checkpw(data.password.encode('utf-8'), user_data['password'].encode('utf-8')):
        raise HTTPException(status_code=ErrorCodeEnum.USER_PASSWORD_SAME.code, detail=ErrorCodeEnum.USER_PASSWORD_SAME.message)
    
    #对新密码进行哈希加密
    new_password = bcrypt.hashpw(data.password.encode('utf-8'),bcrypt.gensalt()).decode('utf-8')
    
    #更新数据库中的密码
    result = await user_service.update_user_password_by_email(data.email,new_password)
    
    if result.modified_count == 0:
        raise HTTPException(status_code=ErrorCodeEnum.USER_PASSWORD_RESET_FAILED.code, detail=ErrorCodeEnum.USER_PASSWORD_RESET_FAILED.message)
    
    return{
        "message": "success",
        "data": {
            "user": result,
        }
    }
    

@router.post("/refresh_token",
summary="刷新token",
description="刷新token接口，刷新token",
response_description="返回刷新后的token"
)
async def refresh_token(request:Request,data: RefreshTokenRequest):
    """刷新token接口"""
    user_service = await get_user_service()
    
    user_data = await user_service.get_user_by_email(data.email)
    
    user_refresh_token = user_data['refresh_token']
    
    if user_refresh_token != data.refresh_token:
        raise HTTPException(status_code=ErrorCodeEnum.USER_REFRESH_TOKEN_INCORRECT.code, detail=ErrorCodeEnum.USER_REFRESH_TOKEN_INCORRECT.message)
    
    #刷新token
    new_access_token = JWTUtils.create_access_token(user_data)
    
    return{
        "message": "success",
        "data": {
            "access_token": new_access_token,
        }
    }


@router.post("/logout",
summary="退出登录",
description="退出登录接口，退出登录",
response_description="返回退出登录结果"
)
async def logout(request:Request,data: LogoutRequest):
    """退出登录接口"""
    user_service = await get_user_service()
    
    await user_service.update_user(data.email,
    {
        "refresh_token":{
        "is_valid":False,
        }
    })
    
    return{
        "message": "success",
    }