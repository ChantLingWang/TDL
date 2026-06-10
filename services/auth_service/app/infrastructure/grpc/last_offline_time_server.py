import os
import sys
import time
from concurrent import futures
import grpc
import asyncio

# 添加项目根目录到 sys.path
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

from infrastructure_sdk.grpc.last_offline_time_grpc.proto import last_offline_time_pb2
from infrastructure_sdk.grpc.last_offline_time_grpc.proto import last_offline_time_pb2_grpc
from app.database.mongodb_user_service import MongoDBUserService


class LastOfflineTimeService(last_offline_time_pb2_grpc.LastOfflineTimeServiceServicer):
    """LastOfflineTimeService gRPC 实现"""

    def __init__(self):
        self.user_service = MongoDBUserService()

    async def UpdateLastOfflineTime(self, request, context):
        """更新用户最后离线时间"""
        user_id = request.user_id

        if not user_id:
            return last_offline_time_pb2.UpdateLastOfflineTimeResponse(
                success=False
            )

        try:
            # 使用当前时间戳（秒）
            current_time = int(time.time())
            
            # 更新到 MongoDB 用户表中
            success = await self.user_service.update_last_offline_time(user_id, current_time)

            return last_offline_time_pb2.UpdateLastOfflineTimeResponse(
                success=success
            )
        except Exception as e:
            print(f"UpdateLastOfflineTime error: {e}")
            return last_offline_time_pb2.UpdateLastOfflineTimeResponse(
                success=False
            )

    async def GetLastOfflineTime(self, request, context):
        """获取用户最后离线时间"""
        user_id = request.user_id

        if not user_id:
            return last_offline_time_pb2.GetLastOfflineTimeResponse(
                last_offline_time=0
            )

        try:
            # 从 MongoDB 获取
            last_time = await self.user_service.get_last_offline_time(user_id)

            if last_time is not None:
                return last_offline_time_pb2.GetLastOfflineTimeResponse(
                    last_offline_time=last_time
                )
            else:
                # 用户没有离线记录
                return last_offline_time_pb2.GetLastOfflineTimeResponse(
                    last_offline_time=0
                )
        except Exception as e:
            print(f"GetLastOfflineTime error: {e}")
            return last_offline_time_pb2.GetLastOfflineTimeResponse(
                last_offline_time=0
            )


def serve(port=50052):
    """启动 gRPC 服务器"""
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    last_offline_time_pb2_grpc.add_LastOfflineTimeServiceServicer_to_server(
        LastOfflineTimeService(), server
    )
    server.add_insecure_port(f'[::]:{port}')
    server.start()
    print(f"gRPC LastOfflineTime Service started on port {port}")
    server.wait_for_termination()


if __name__ == '__main__':
    serve()
