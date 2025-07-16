import asyncio

from services.mongodb_service import MongoDBService


async def main():
    #创建数据库服务实例
    db_service = MongoDBService()
    try:
        #连接数据库
        await db_service.connect()
        #测试连接数据库
        if db_service.is_connected:
            print("数据库已连接")
        else:
            print("数据连接失败")
        user_connection = db_service.get_collection("users")
        print("获取到user集合")
    except Exception as e:
        print(f"操作失败,{e}")
    finally:
        db_service.close()
        if not db_service.is_connected:
            print("数据库已关闭")

if __name__ == "__main__":
    asyncio.run(main())