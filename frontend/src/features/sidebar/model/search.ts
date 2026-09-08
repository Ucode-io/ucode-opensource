import type { MenuNode } from "./types";

/**
 * Найденный пункт вместе с дорогой до него.
 *
 * Дорога — это пары «id и подпись» родительских папок от корня. Подпись
 * показывается под именем (без неё две «Заявки» из разных папок
 * неразличимы), а id нужны, чтобы по щелчку раскрыть дерево до найденного.
 */
export type MenuMatch = {
  node: MenuNode;
  trail: { id: string; label: string }[];
};

/**
 * Отбор пунктов по набранному.
 *
 * Ищем в подписи на ВСЕХ языках проекта, а не только в показанной:
 * в проекте на трёх языках человек помнит пункт под тем именем, которое
 * видел, а видел он его на своём языке интерфейса.
 *
 * Совпадения с начала имени идут первыми: набравший «за» ищет «Заявки»,
 * а не «Обработка заявок». Внутри каждой из двух групп порядок остаётся
 * тот же, что в дереве, — sort в JS устойчив.
 */
export function matchMenus(items: MenuMatch[], query: string): MenuMatch[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return [];

  const names = (match: MenuMatch) => [
    match.node.label,
    ...Object.values(match.node.labels),
  ];

  return items
    .filter((match) =>
      names(match).some((name) => name.toLowerCase().includes(needle)),
    )
    .sort((a, b) => Number(startsWith(names(b), needle)) - Number(startsWith(names(a), needle)));
}

function startsWith(names: string[], needle: string): boolean {
  return names.some((name) => name.toLowerCase().startsWith(needle));
}
