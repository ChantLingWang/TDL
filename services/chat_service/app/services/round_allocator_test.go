package services

import (
	"testing"

	"chat_service/app/api/models"
)

func TestRoundAllocatorNextRound(t *testing.T) {
	allocator := &RoundAllocator{rounds: make(map[string]int64)}

	if got := allocator.nextRound("conv-a"); got != 1 {
		t.Fatalf("conv-a 第一轮应为 1, got %d", got)
	}
	if got := allocator.nextRound("conv-a"); got != 2 {
		t.Fatalf("conv-a 第二轮应为 2, got %d", got)
	}
	if got := allocator.nextRound("conv-b"); got != 1 {
		t.Fatalf("conv-b 第一轮应为 1, got %d", got)
	}
	if got := allocator.nextRound("conv-a"); got != 3 {
		t.Fatalf("conv-a 第三轮应为 3, got %d", got)
	}
}

func TestBuildPartsText(t *testing.T) {
	parts, err := buildParts(models.ChatMessageRequest{
		Parts: []models.ChatPart{{Type: "text", Text: "hi"}},
	})
	if err != nil {
		t.Fatalf("buildParts 失败: %v", err)
	}
	if len(parts) != 1 {
		t.Fatalf("parts 数量错误: %d", len(parts))
	}
	if parts[0].GetType() != "text" || parts[0].GetText() != "hi" {
		t.Fatalf("text part 错误: %v", parts[0])
	}
}

func TestBuildPartsRejectsTwoText(t *testing.T) {
	_, err := buildParts(models.ChatMessageRequest{
		Parts: []models.ChatPart{
			{Type: "text", Text: "a"},
			{Type: "text", Text: "b"},
		},
	})
	if err == nil {
		t.Fatal("两个 text part 应该报错")
	}
}

func TestBuildPartsRejectsTextNotLast(t *testing.T) {
	_, err := buildParts(models.ChatMessageRequest{
		Parts: []models.ChatPart{
			{Type: "text", Text: "a"},
			{Type: "image", Data: []byte{1}, MimeType: "image/png"},
		},
	})
	if err == nil {
		t.Fatal("text 不在最后应该报错")
	}
}

func TestBuildPartsImageMetadata(t *testing.T) {
	parts, err := buildParts(models.ChatMessageRequest{
		Parts: []models.ChatPart{{
			Type:     "image",
			Data:     []byte{1, 2, 3},
			MimeType: "image/png",
			Metadata: map[string]interface{}{"width": float64(1024), "probe": "x"},
		}},
	})
	if err != nil {
		t.Fatalf("buildParts 失败: %v", err)
	}
	if len(parts) != 1 || parts[0].GetType() != "image" {
		t.Fatalf("image part 错误: %v", parts)
	}
	meta := parts[0].GetMetadata().AsMap()
	if meta["width"] != float64(1024) || meta["probe"] != "x" {
		t.Fatalf("metadata 错误: %v", meta)
	}
}
