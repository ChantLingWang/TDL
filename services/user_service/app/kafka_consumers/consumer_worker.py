"""
消费者工作进程 - 独立运行的消费者进程入口
"""
import asyncio
import logging
import signal
import sys
from typing import Dict, Any

from app.kafka_consumers.kafka_consumer import kafka_consumer
from app.kafka_consumers.event_handlers import event_handlers

logger = logging.getLogger(__name__)


class ConsumerWorker:
    """消费者工作进程类"""
    
    def __init__(self):
        self.running = False
        # 注册信号处理器
        signal.signal(signal.SIGINT, self._signal_handler)
        signal.signal(signal.SIGTERM, self._signal_handler)
    
    def _signal_handler(self, signum, _frame):
        """处理系统信号"""
        logger.info(f"接收到信号 {signum}")
        self.running = False
    
    def _register_handlers(self):
        """注册事件处理器"""
        kafka_consumer.register_handler("register_events", self._handle_user_registered_event)
    
    def _handle_user_registered_event(self, event_data: Dict[str, Any]) -> None:
        """处理用户注册事件"""
        asyncio.run(event_handlers.handle_user_registered_event(event_data))
    
    def _setup_logging(self):
        """配置日志"""
        logging.basicConfig(
            level=logging.INFO,
            format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
            handlers=[logging.StreamHandler(sys.stdout), logging.FileHandler('consumer.log')]
        )
    
    def run(self):
        """运行消费者"""
        self._setup_logging()
        logger.info("启动消费者工作进程")
        
        try:
            self._register_handlers()
            kafka_consumer.subscribe(["register_events"])
            self.running = True
            kafka_consumer.start_consuming()
        except Exception as e:
            logger.error(f"消费者异常: {e}")
            raise
        finally:
            self.cleanup()
    
    def cleanup(self):
        """清理资源"""
        kafka_consumer.stop()


def main():
    worker = ConsumerWorker()
    try:
        worker.run()
    except KeyboardInterrupt:
        pass
    except Exception as e:
        logger.error(f"工作进程失败: {e}")
        sys.exit(1)
    finally:
        worker.cleanup()


if __name__ == "__main__":
    main()