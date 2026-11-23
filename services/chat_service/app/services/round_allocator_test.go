package services

import "testing"

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
