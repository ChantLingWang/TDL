from fastapi import APIRouter, HTTPException, Depends
from services.auth_service.app.models.auth_model import RegisterRequest,LoginRequest,SendCodeRequest,VerifyCodeRequest
from services.auth_service.app.database.mongodb_user_service import MongoDBUserService,db_manager
from services.auth_service.app.database.redis_user_service import RedisUserService
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
        raise HTTPException(status_code=500, detail=ErrorCodeEnum.DATABASE_CONNECTION_ERROR.message)
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
        raise HTTPException(status_code=500, detail=ErrorCodeEnum.EMAIL_SEND_ERROR.message)


@router.post("/verify_code_register",
    summary="验证注册验证码",
    description="验证注册验证码接口，验证注册验证码",
    response_description="返回验证结果"
)
async def verify_code_register(request:Request,data: VerifyCodeRequest):
    """验证注册验证码接口"""
    redis_client = RedisUserService()
    code = redis_client.get_code(data.email)
    
    #创建载荷
    user = {
        "email": data.email,
    }
    
    #生成token
    access_token = JWTUtils.create_access_token(user)
    refresh_token = JWTUtils.create_refresh_token(user)
    
    if code is None:
        raise HTTPException(status_code=400, detail=ErrorCodeEnum.USER_VERIFICATION_CODE_EXPIRED.message)
    if code != data.code:
        raise HTTPException(status_code=400, detail=ErrorCodeEnum.USER_VERIFICATION_CODE_INCORRECT.message)
    return {
        "message": "success",
        "data": {
            "email": data.email,
            "access_token": access_token,
            "refresh_token": refresh_token
        }
    }
    
    
@router.post("/register",
    summary="用户注册",
    description="用户注册接口，创建新用户账户",
    response_description="返回注册结果"
)
async def register(request:Request,data: RegisterRequest):
    """用户注册接口"""
    
    user_service = await get_user_service()
        
    # 检查用户是否已存在
    existing_user = await user_service.get_user_by_email(data.email)
    if existing_user:
        raise HTTPException(status_code=409, detail=ErrorCodeEnum.USER_ALREADY_EXISTS.message)
        
    # 创建新用户
    user_data = {
        "username": data.username,
        "email": data.email,
        "password": bcrypt.hashpw(data.password.encode('utf-8'),bcrypt.gensalt())
    }
    user = await user_service.create_user(user_data)
    
    #生成token
    access_token = JWTUtils.create_access_token(user)
    refresh_token = JWTUtils.create_refresh_token(user)
    
    return {
        "message": "success",
        "data": {
            "user": {
                "username": user["username"],
                "email": user["email"],
                "created_at": user.get("created_at"),
                "access_token": access_token,
                "refresh_token": refresh_token
            }
        }
    }


@router.post("/login",
    summary="用户登录",
    description="用户登录接口，验证用户凭据",
    response_description="返回登录结果"
)
async def login(request:Request,data: LoginRequest):
    """用户登录接口"""
    
    user_service = await get_user_service()
    
    # 检查用户是否存在
    user = await user_service.get_user_by_email(data.email)
    if not user:
        raise HTTPException(status_code=404, detail=ErrorCodeEnum.USER_NOT_FOUND.message)
    
    # 验证密码
    if not bcrypt.checkpw(data.password.encode('utf-8'),user['password']):
        raise HTTPException(status_code=401, detail=ErrorCodeEnum.USER_PASSWORD_INCORRECT.message)
    
    # 生成token
    access_token = JWTUtils.create_access_token(user)
    refresh_token = JWTUtils.create_refresh_token(user)
    return {
        "message": "success",
        "data": {
            "user": {
                "email": user["email"],
                "created_at": user.get("created_at"),
                "access_token": access_token,
                "refresh_token": refresh_token
            }
        }
    }
        
