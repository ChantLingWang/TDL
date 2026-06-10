import os
import sys
from concurrent import futures
import grpc

# 添加项目根目录到 sys.path
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from infrastructure_sdk.grpc.token_auth_grpc.proto import auth_pb2
from infrastructure_sdk.grpc.token_auth_grpc.proto import auth_pb2_grpc
from app.services.jwt_service import JWTUtils
from app.database.mongodb_user_service import MongoDBUserService
from app.database.mongodb_service import db_manager


class AuthService(auth_pb2_grpc.AuthServiceServicer):
    """AuthService gRPC 实现"""

    def VerifyToken(self, request, context):
        """验证 Token 并返回用户信息"""
        token = request.token

        # 使用 JWTUtils 验证 token
        result = JWTUtils.verify_token(token)

        if result.get("status") == "success":
            # 使用 user_info 提取用户信息（包含 userid, username, email）
            user_info = result.get("user_info", {})
            return auth_pb2.VerifyTokenResponse(
                valid=True,
                user_id=user_info.get("user_id", ""),
                username=user_info.get("username", ""),
                email=user_info.get("email", ""),
                message="token is valid"
            )
        else:
            return auth_pb2.VerifyTokenResponse(
                valid=False,
                message=result.get("message", "invalid token")
            )

    def GetUserByID(self, request, context):
        """根据 user_id 字符串查询用户信息"""
        user_id = request.user_id
        user_service = MongoDBUserService(db_manager)

        try:
            user = user_service.get_user_by_user_id(user_id)
        except Exception as e:
            return auth_pb2.GetUserByIDResponse(
                found=False,
                message=str(e),
            )

        if user is None:
            return auth_pb2.GetUserByIDResponse(
                found=False,
                message="user not found",
            )

        return auth_pb2.GetUserByIDResponse(
            found=True,
            user_id=user.get("user_id", ""),
            username=user.get("username", ""),
            email=user.get("email", ""),
            status=user.get("status", ""),
        )


def serve(port=50051):
    """启动 gRPC 服务器"""
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    auth_pb2_grpc.add_AuthServiceServicer_to_server(AuthService(), server)
    server.add_insecure_port(f'[::]:{port}')
    server.start()
    print(f"gRPC Auth Service started on port {port}")
    server.wait_for_termination()


if __name__ == '__main__':
    serve()
