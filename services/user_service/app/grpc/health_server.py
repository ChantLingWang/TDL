import asyncio
import grpc

from . import health_pb2, health_pb2_grpc


class HealthServicer(health_pb2_grpc.HealthServicer):
    async def Check(self, request, context):
        return health_pb2.HealthCheckResponse(
            status=health_pb2.HealthCheckResponse.SERVING,
            message="user_service ok"
        )

    async def Watch(self, request, context):
        yield health_pb2.HealthCheckResponse(
            status=health_pb2.HealthCheckResponse.SERVING,
            message="user_service ok"
        )


async def serve(address: str = "0.0.0.0:50052"):
    server = grpc.aio.server()
    health_pb2_grpc.add_HealthServicer_to_server(HealthServicer(), server)
    server.add_insecure_port(address)
    await server.start()
    print(f"[gRPC] user_service health server started at {address}")
    await server.wait_for_termination()


if __name__ == "__main__":
    asyncio.run(serve()) 