// dedupeById 按 id 去重并保持先出现的顺序。
// 热榜按 offset 翻页，跨分钟重建快照时相邻两页可能重叠，拼接后用它兜住
export function dedupeById<T extends { id: number }>(items: T[]): T[] {
  const seen = new Set<number>();
  const result: T[] = [];
  for (const item of items) {
    if (seen.has(item.id)) continue;
    seen.add(item.id);
    result.push(item);
  }
  return result;
}
