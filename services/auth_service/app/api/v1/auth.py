from fastapi import APIRouter, HTTPException, Depends
from models.auth_model import LoginOrRegisterRequest
from database.mongodb_user_service import MongoDBUserService, db_manager

router = APIRouter()

@router.post("/login_or_register",
    summary="用户登录或注册",
    description="用户登录或注册接口，如果用户不存在则自动注册",
    response_description="返回登录或注册结果"
)
async def login_or_register(data: LoginOrRegisterRequest):
    """用户登录或注册接口"""
    try:
        # 检查数据库连接
        is_connected = await db_manager.test_connection()
        if not is_connected:
            raise HTTPException(status_code=500, detail="数据库连接失败")
        
        # 创建用户服务实例
        user_service = MongoDBUserService(db_manager)
        
        # 执行登录或注册逻辑
        user = await user_service.login_or_register_user(data.email, data.password)
        
        # 返回用户信息（不包含敏感信息）
        return {
            "message": "success",
            "data": {
                "user": {
                    "email": user["email"],
                    "created_at": user.get("created_at"),
                    "updated_at": user.get("updated_at")
                }
            }
        }
        
    except ValueError as e:
        # 业务逻辑错误（如密码错误）
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        # 系统错误
        raise HTTPException(status_code=500, detail=f"系统错误: {str(e)}")

@router.post("/login",
    summary="用户登录",
    description="用户登录接口",
    response_description="返回登录结果"
)
async def login(data: LoginOrRegisterRequest):
    """用户登录接口"""
    try:
        # 检查数据库连接
        is_connected = await db_manager.test_connection()
        if not is_connected:
            raise HTTPException(status_code=500, detail="数据库连接失败")
        
        # 创建用户服务实例
        user_service = MongoDBUserService(db_manager)
        
        # 检查用户是否存在
        user = await user_service.get_user_by_email(data.email)
        if not user:
            raise HTTPException(status_code=404, detail="用户不存在")
        
        # 验证密码（这里需要在user_service中实现密码验证方法）
        # 临时使用login_or_register_user方法
        user = await user_service.login_or_register_user(data.email, data.password)
        
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
        
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"登录失败: {str(e)}") 