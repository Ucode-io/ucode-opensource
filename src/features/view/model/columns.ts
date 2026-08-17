import type { Field } from "@/features/table";
import { isTabView, type View } from "./types";

/**
 * Колонки view → поля таблицы, в порядке, который задал view.
 *
 * Ключевая тонкость: view перечисляет колонки идентификаторами, и для
 * полей-связей это id СВЯЗИ, а не id поля. Поэтому индекс строится по
 * двум ключам. Без этого у таблицы с пятью связями молча пропадают
 * пять колонок — они просто не находятся.
 *
 * Второй раз одно и то же поле не возвращается. У связи два ключа, и
 * в columns попадают оба — id поля и id связи; бэкенд дописывает их
 * туда независимо. Дубль в списке — это два React-ключа `column.id`
 * на одну колонку: React оставляет в DOM ячейки прошлого view, и после
 * перехода по вкладкам шапка растёт, а меню колонки перестаёт
 * открываться (клик приходит по осиротевшему заголовку).
 *
 * Чистая функция: проверяется тестом, а не открыванием экрана.
 */
export function resolveColumns(view: View | undefined, fields: Field[]): Field[] {
  return view ? resolveColumnIds(view.columnIds, fields) : [];
}

/**
 * То же самое, но от голого списка идентификаторов.
 *
 * Отдельно, потому что колонки перечисляет не только view: у вкладки
 * связи в карточке свой список, и он приходит из раскладки
 * (`tabs[].relation.columns`). Правило разбора при этом одно на всех —
 * второго места, где id превращаются в поля, быть не должно.
 */
export function resolveColumnIds(columnIds: string[], fields: Field[]): Field[] {
  const index = new Map<string, Field>();
  for (const field of fields) {
    index.set(field.id, field);
    if (field.relationId) index.set(field.relationId, field);
  }

  // Неизвестный id — поле удалили, а из view его не вычистили. Пропускаем:
  // пустая колонка без заголовка хуже отсутствующей.
  const seen = new Set<string>();
  const columns: Field[] = [];

  for (const id of columnIds) {
    const field = index.get(id);
    if (!field || seen.has(field.id)) continue;

    seen.add(field.id);
    columns.push(field);
  }

  return columns;
}

/**
 * Ключ поля в `view.columns`: у связи это id СВЯЗИ, а не поля.
 *
 * Обратная сторона resolveColumns: тем же ключом колонка и записывается.
 * Одно имя — одно место, иначе порядок, собранный по id поля, перестаёт
 * находиться по id связи, и колонка-ссылка молча пропадает.
 */
export function columnKey(field: Field): string {
  return field.relationId ?? field.id;
}

/**
 * Перенос ключа на место другого — перестановка колонок мышью.
 *
 * Перетащенный встаёт ПЕРЕД целью и при движении вверх, и при движении
 * вниз: одно правило вместо двух означает, что бросок на одну и ту же
 * строку всегда даёт один и тот же результат, куда бы ни ехали.
 */
export function moveBefore(keys: string[], moved: string, target: string): string[] {
  if (moved === target) return keys;

  const rest = keys.filter((key) => key !== moved);
  const at = rest.indexOf(target);
  // Цели в списке нет — бросили мимо; порядок не трогаем.
  if (at < 0) return keys;

  return [...rest.slice(0, at), moved, ...rest.slice(at)];
}

/**
 * Вкладки экрана: только те view, у которых есть свой экран. Порядок
 * задаёт админ; TABLE приходит без order, поэтому сортировка устойчивая
 * — равные остаются в порядке ответа сервера.
 */
export function tabViews(views: View[]): View[] {
  return views.filter(isTabView).sort((a, b) => a.order - b.order);
}


/**
 * Какой view открыт. Явно выбранный из адреса, иначе первая вкладка.
 *
 * Ссылка на удалённый view не должна давать пустой экран: если id
 * не нашёлся, открывается первая вкладка, как будто его и не просили.
 */
export function pickView(views: View[], viewId: string | undefined): View | undefined {
  const tabs = tabViews(views);
  return tabs.find((view) => view.id === viewId) ?? tabs[0];
}
