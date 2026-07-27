package retriever

type ContextItem interface{}

// ReorderSandwichContext 首尾强化组装算法 (防止 LLM 对长 Context 中间信息失明 Lost in the Middle)
// 将排名最高 (得分最高) 的块放在 index 0，第 2 高的块放在最后一个 index，其余块填在中间。
func ReorderSandwichContext[T any](items []T) []T {
	n := len(items)
	if n <= 2 {
		return items
	}

	reordered := make([]T, n)
	// 第 1 高相关度块放在开头 (index 0)
	reordered[0] = items[0]
	// 第 2 高相关度块放在末尾 (index n-1)
	reordered[n-1] = items[1]

	// 剩余块填入中间位置
	for i := 2; i < n; i++ {
		reordered[i-1] = items[i]
	}

	return reordered
}
