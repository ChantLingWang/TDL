from collections import UserDict


class ErrorCode():
    def __init__(self,code:int,message:str):
        self._code = code
        self._message = message

    @property
    def code(self):
        return self._code
    
    @property
    def message(self):
        return self._message


class ErrorCodeEnum:
    #用户登录注册相关错误码
    USER_NOT_FOUND = ErrorCode(10001,"邮箱错误")
    USER_ALREADY_EXISTS = ErrorCode(10002,"用户已存在")
    USER_PASSWORD_INCORRECT = ErrorCode(10003,"密码错误")
    USER_VERIFICATION_CODE_INCORRECT = ErrorCode(10004,"验证码错误")
    USER_VERIFICATION_CODE_EXPIRED = ErrorCode(10005,"验证码已过期或不存在")
    
    #数据库相关错误码
    DATABASE_CONNECTION_ERROR = ErrorCode(20001,"数据库连接失败")
    
    #邮箱相关错误码
    EMAIL_SEND_ERROR = ErrorCode(30001,"邮件发送失败,请稍后重试")
    
