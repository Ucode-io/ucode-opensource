/**
 * Перенос ключа на место другого — перестановка мышью.
 *
 * Перетащенный встаёт ПЕРЕД целью и при движении вверх, и при движении
 * вниз: одно правило вместо двух означает, что бросок на одну и ту же
 * строку всегда даёт один и тот же результат, куда бы ни ехали.
 *
 * В shared, а не в фиче: так переставляют и колонки view, и графики
 * на экране CHART, а фича у фичи ничего, кроме index.ts, не берёт.
 */
export function moveBefore(keys: string[], moved: string, target: string): string[] {
  if (moved === target) return keys;

  const rest = keys.filter((key) => key !== moved);
  const at = rest.indexOf(target);
  // Цели в списке нет — бросили мимо; порядок не трогаем.
  if (at < 0) return keys;

  return [...rest.slice(0, at), moved, ...rest.slice(at)];
}
