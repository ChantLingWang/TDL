from fastapi import APIRouter, HTTPException, Depends
from services.auth_service.app.models.auth_model import RegisterRequest,LoginRequest
from services.auth_service.app.database.mongodb_user_service import MongoDBUserService,db_manager
from fastapi import Request
import bcrypt

router = APIRouter()

async def get_user_service():
    """获取用户服务实例并检查数据库连接"""
    is_connected = await db_manager.test_connection()
    if not is_connected:
        raise HTTPException(status_code=500, detail="数据库连接失败")
    return MongoDBUserService(db_manager)

@router.post("/register",
    summary="用户注册",
    description="用户注册接口，创建新用户账户",
    response_description="返回注册结果"
)
async def register(request:Request,data: RegisterRequest):
    """用户注册接口"""
    try:
        user_service = await get_user_service()
        
        # 检查用户是否已存在
        existing_user = await user_service.get_user_by_email(data.email)
        if existing_user:
            raise HTTPException(status_code=409, detail="用户已存在")
        
        # 创建新用户
        user_data = {
            "username": data.username,
            "email": data.email,
            "password": bcrypt.hashpw(data.password.encode('utf-8'),bcrypt.gensalt())
        }
        user = await user_service.create_user(user_data)
        
        return {
            "message": "注册成功",
            "data": {
                "user": {
                    "username": user["username"],
                    "email": user["email"],
                    "created_at": user.get("created_at"),
                    "updated_at": user.get("updated_at")
                }
            }
        }
        
    except HTTPException:
        raise
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"注册失败: {str(e)}")



@router.post("/login",
    summary="用户登录",
    description="用户登录接口，验证用户凭据",
    response_description="返回登录结果"
)
async def login(data: LoginRequest):
    """用户登录接口"""
    try:
        user_service = await get_user_service()
        
        # 检查用户是否存在
        user = await user_service.get_user_by_email(data.email)
        if not user:
            raise HTTPException(status_code=404, detail="用户不存在")
        
        # 验证密码
        is_valid = await user_service.verify_password(data.email, data.password)
        if not is_valid:
            raise HTTPException(status_code=401, detail="密码错误")
        
        return {
            "message": "登录成功",
            "data": {
                "user": {
                    "email": user["email"],
                    "created_at": user.get("created_at"),
                    "updated_at": user.get("updated_at")
                }
            }
        }
        
    except HTTPException:
        raise
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"登录失败: {str(e)}")
