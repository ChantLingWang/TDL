from fastapi import APIRouter


router = APIRouter()

@router.get("/get_user_info")
async def get_user_info(user_id: str):
    """获取用户信息"""
    user = await get_user_service().get_user(user_id)
    return user