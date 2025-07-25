from fastapi import APIRouter, HTTPException, Depends
from models.auth_model import RegisterRequest,LoginRequest
from database.mongodb_user_service import MongoDBUserService,db_manager
from utils.error_code import ErrorCodeEnum
from fastapi import Request
import bcrypt


router = APIRouter()


async def get_user_service():
    """获取用户服务实例并检查数据库连接"""
    is_connected = await db_manager.test_connection()
    if not is_connected:
        raise HTTPException(status_code=500, detail=ErrorCodeEnum.DATABASE_CONNECTION_ERROR.message)
    return MongoDBUserService(db_manager)


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
        
    return {
        "message": "success",
        "data": {
            "user": {
                "username": user["username"],
                "email": user["email"],
                "created_at": user.get("created_at"),
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
    
    return {
        "message": "success",
        "data": {
            "user": {
                "email": user["email"],
                "created_at": user.get("created_at"),
            }
        }
    }
        
