import asyncio
import grpc

from . import health_pb2, health_pb2_grpc


async def check_health(target: str):
    async with grpc.aio.insecure_channel(target) as channel:
        stub = health_pb2_grpc.HealthStub(channel)
        resp = await stub.Check(health_pb2.HealthCheckRequest(service=""))
        return resp.status, resp.message


if __name__ == "__main__":
    # 示例：检查 user_service 的健康（假设在50052端口）
    status, message = asyncio.run(check_health("localhost:50052"))
    print("health status:", status, "message:", message) 