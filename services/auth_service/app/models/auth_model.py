import email
from pydantic import BaseModel,Field,EmailStr

class LoginRequest(BaseModel):
    email: EmailStr = Field(..., description="用户邮箱")
    password: str = Field(..., description="用户密码")
    
class SendCodeRequest(BaseModel):
    email: EmailStr = Field(..., description="用户邮箱")
    
class VerifyCodeRequest(BaseModel):
    username: str = Field(..., min_length=3, max_length=10,description="用户名")
    email: EmailStr = Field(..., description="用户邮箱")
    password: str = Field(..., description="用户密码")
    code: str = Field(..., min_length=6, max_length=6,description="验证码")

class VerifyCodeLoginRequest(BaseModel):
    email: EmailStr = Field(..., description="用户邮箱")
    code: str = Field(..., min_length=6, max_length=6,description="验证码")

class ResetPasswordRequest(BaseModel):
    user_id: str = Field(..., description="用户id")
    email: EmailStr = Field(..., description="用户邮箱")
    code: str = Field(..., min_length=6, max_length=6,description="验证码")
    password: str = Field(..., description="用户密码")

class RefreshTokenRequest(BaseModel):
    user_id: str = Field(..., description="用户id")
    refresh_token: str = Field(..., description="刷新token")
    email: EmailStr = Field(..., description="用户邮箱")

class LogoutRequest(BaseModel):
    user_id: str = Field(..., description="用户id")
    refresh_token: str = Field(..., description="刷新token")