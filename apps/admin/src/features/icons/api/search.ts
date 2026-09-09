import { useQuery } from "@tanstack/react-query";
import { keys } from "@/shared/lib/query-keys";

/**
 * Поиск иконок в Iconify, ограниченный коллекцией Tabler.
 *
 * Ограничение — не придирка: Tabler целиком контурный, поэтому любая
 * выбранная иконка автоматически совпадает по стилю с остальным
 * интерфейсом. Разрешить все коллекции — значит снова получить смесь
 * заливных и контурных, которую потом ничем не выровнять.
 *
 * Иконки не попадают в бандл: имя хранится строкой, картинка грузится
 * по требованию тем же загрузчиком, что и остальные иконки из данных.
 */
const ICONIFY = "https://api.iconify.design";
const PREFIX = "tabler";
// Просим с запасом: часть результатов отсеется как заливная.
const LIMIT = 96;

/**
 * В коллекции Tabler заливные варианты помечены суффиксом -filled
 * (`tabler:home` и `tabler:home-filled`). Их отсеиваем: смысл ограничения
 * коллекцией — единый контурный стиль, а заливные его ломают.
 */
export const isOutline = (icon: string) => !icon.endsWith("-filled");

type SearchResponse = { icons?: string[] };
type CollectionResponse = {
  uncategorized?: string[];
  categories?: Record<string, string[]>;
};

async function fetchIcons(query: string, signal: AbortSignal): Promise<string[]> {
  if (query.trim()) {
    const url = `${ICONIFY}/search?query=${encodeURIComponent(query)}&prefix=${PREFIX}&limit=${LIMIT}`;
    const data = (await fetch(url, { signal }).then((r) => r.json())) as SearchResponse;
    return (data.icons ?? []).filter(isOutline);
  }

  // Пустой запрос — показываем начало коллекции, а не пустую сетку.
  const data = (await fetch(`${ICONIFY}/collection?prefix=${PREFIX}`, { signal }).then((r) =>
    r.json(),
  )) as CollectionResponse;

  const fromCategories = Object.values(data.categories ?? {}).flat();
  const names = fromCategories.length > 0 ? fromCategories : (data.uncategorized ?? []);

  return names
    .filter(isOutline)
    .slice(0, LIMIT)
    .map((name) => `${PREFIX}:${name}`);
}

export function useIconSearch(query: string) {
  return useQuery({
    queryKey: keys.icons.search(query),
    queryFn: ({ signal }) => fetchIcons(query, signal),
    // Набор иконок не меняется — держим на весь сеанс.
    staleTime: Infinity,
    retry: 1,
  });
}
