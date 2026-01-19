from typing import Optional, Any, Dict
from pydantic import BaseModel

class SagaStepResult(BaseModel):
    """Saga步骤执行结果"""
    success: bool
    output_data: Optional[Dict[str, Any]] = None
    error_message: Optional[str] = None

    @classmethod
    def success(cls, output_data: Dict[str, Any] = None) -> "SagaStepResult":
        return cls(success=True, output_data=output_data or {})

    @classmethod
    def failure(cls, error_message: str) -> "SagaStepResult":
        return cls(success=False, error_message=error_message)
